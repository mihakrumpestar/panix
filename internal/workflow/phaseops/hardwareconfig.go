package phaseops

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// ErrWriteModeGenerator reports a nixos-generate-config generator that mutates
// the target instead of printing a config panix can capture.
var ErrWriteModeGenerator = errors.New(
	"nixos-generate-config runs without --show-hardware-config, so it writes the target's " +
		"/etc/nixos files instead of printing the config; add the flag to hardware_config.generate " +
		"or configure a different generator",
)

// stageMu serializes the git index operations: machines run concurrently and git
// takes .git/index.lock, so parallel staging would fail all but one.
var stageMu sync.Mutex

// GenerateHardwareConfig writes the target's generated hardware config to
// HardwareConfig.Path, a path on the machine running panix. No-op when that path
// is empty; the target must already run the installer.
func GenerateHardwareConfig(exc *executioner.Executioner, machineI *machine.Machine) error {
	if machineI.HardwareConfig.Path == "" {
		return nil
	}

	// The single choke point for every caller: a write-mode generator rewrites
	// the target's /etc/nixos as root and prints nothing panix can capture.
	if IsWriteModeGenerator(machineI.HardwareConfig.Generate) {
		return ErrWriteModeGenerator
	}

	argv := append(machineI.MaybeSudo(), machineI.HardwareConfig.Generate.Command...)
	argv = append(argv, machineI.HardwareConfig.Generate.Args...)

	err := exc.Exec(
		"generate config",
		"generating hardware config",
		strings.Join(machineI.HardwareConfig.Generate.Command, " ")+" failed",
		argv,
		executioner.OnSuccess(func(commandLog *command.CommandLog) error {
			writeErr := writeHardwareConfig(machineI.HardwareConfig.Path, commandLog.Output.String())
			if writeErr != nil {
				return writeErr
			}

			log.Info().
				Str("xpath", machineI.Xpath.String()).
				Str("hardware_config_path", machineI.HardwareConfig.Path).
				Msg("wrote hardware config locally")

			stageHardwareConfig(machineI.HardwareConfig.Path)

			return nil
		}),
		executioner.OnDryRun(func() {}),
	)
	if err != nil {
		return errors.Wrap(err, "hardware config generation failed")
	}

	return nil
}

// writeHardwareConfig validates captured output before writing it to path,
// creating parent directories as needed.
func writeHardwareConfig(path, output string) error {
	content, err := hardwareConfigFromOutput(output)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	err = os.MkdirAll(dir, filepermissions.DefaultDirPermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create hardware config directory %s", dir)
	}

	err = os.WriteFile(path, content, filepermissions.DefaultFilePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write hardware config to %s", path)
	}

	return nil
}

// stageHardwareConfig marks path for addition in its git work tree so the build
// that follows evaluates the file: Nix snapshots only git-visible paths.
// Best effort by design so staging never fails a deploy.
func stageHardwareConfig(path string) {
	stageMu.Lock()
	defer stageMu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	dir := filepath.Dir(absPath)
	if !isGitWorkTree(dir) {
		return
	}

	err = gitRun(dir, "add", "-N", "--", absPath)
	if err == nil {
		log.Info().Str("path", path).Msg("staged the hardware config so the flake sees it")

		return
	}

	// Generated hardware configs are usually gitignored, and ignored paths reject plain -N.
	err = gitRun(dir, "add", "-fN", "--", absPath)
	if err != nil {
		log.Warn().
			Err(err).
			Str("path", path).
			Msgf("could not stage the hardware config, run git -C %s add -fN -- %s manually",
				shellquote.QuoteWord(dir), shellquote.QuoteWord(absPath))

		return
	}

	log.Info().Str("path", path).Msg("force-staged the gitignored hardware config so the flake sees it")
}

// isGitWorkTree reports whether dir is inside a non-bare git work tree. A
// failure also covers git not being on PATH: there is nothing to stage.
func isGitWorkTree(dir string) bool {
	//nolint:gosec // dir is an operator-configured path, not user input
	out, err := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(out)) == "true"
}

// gitRun runs git in dir and reports the exit status; no output is consumed.
// Staging is a local action with no caller context to honor.
func gitRun(dir string, args ...string) error {
	//nolint:gosec // args are operator-configured values, not user input
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)

	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "git %s failed", strings.Join(args, " "))
	}

	return nil
}

// IsWriteModeGenerator reports the generator shape that mutates the target
// instead of printing a config: nixos-generate-config only prints when asked.
// The command is compared by base name at any position, so a wrapper or an
// absolute path is caught too, and the flag may sit in the command or the args.
func IsWriteModeGenerator(generator attributes.Generator) bool {
	if !CommandInvokes(generator.Command, "nixos-generate-config") {
		return false
	}

	return !slices.Contains(slices.Concat(generator.Command, generator.Args), "--show-hardware-config")
}

// hardwareConfigFromOutput validates captured output before it is written: the
// executioner merges stderr into the same PTY stream, so only a recognized
// generator result may pass. Accepted shapes are a NixOS module
// (nixos-generate-config) and a JSON object (nixos-facter); anything else is
// captured stderr noise. The content ends with exactly one newline.
func hardwareConfigFromOutput(output string) ([]byte, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, errors.New("hardware config generator produced no output, stderr is merged into the captured PTY stream, " +
			"so a warning or error may be all that was captured")
	}

	firstLine, _, _ := strings.Cut(trimmed, "\n")

	switch {
	case strings.HasPrefix(trimmed, "{"):
		// A module can also start with `{`, so a JSON parse failure falls
		// through to the module checks instead of rejecting it here.
		err := validateJSONReport(trimmed, firstLine)
		if err == nil {
			return []byte(trimmed + "\n"), nil
		}

		err = validateNixModule(trimmed, firstLine)
		if err != nil {
			return nil, errors.Errorf("unexpected hardware config output (first line %q), it is neither a JSON report "+
				"nor a NixOS module, so the captured PTY stream was cut off or polluted by stderr", firstLine)
		}
	case strings.HasPrefix(firstLine, "#"):
		err := validateNixModule(trimmed, firstLine)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("unexpected hardware config output (first line %q), stderr is merged into the "+
			"captured PTY stream, so the target printed a warning or error instead of the config", firstLine)
	}

	return []byte(trimmed + "\n"), nil
}

// validateJSONReport accepts a JSON object emitted by a report generator such as
// nixos-facter. Parsing is the check: a truncated or stderr-polluted capture
// fails it.
func validateJSONReport(trimmed, firstLine string) error {
	var report map[string]any

	err := json.Unmarshal([]byte(trimmed), &report)
	if err != nil {
		return errors.Errorf("unexpected hardware config output (first line %q), it starts like a JSON report "+
			"but does not parse, so the captured PTY stream was cut off or polluted by stderr: %v", firstLine, err)
	}

	if len(report) == 0 {
		return errors.Errorf("unexpected hardware config output (first line %q), the JSON report is empty", firstLine)
	}

	return nil
}

// validateNixModule accepts nixos-generate-config's --show-hardware-config
// output.
func validateNixModule(trimmed, firstLine string) error {
	if !strings.Contains(trimmed, "modulesPath") || !strings.Contains(trimmed, "config,") {
		return errors.Errorf("unexpected hardware config output (first line %q), the captured output is not "+
			"a hardware config", firstLine)
	}

	if !strings.HasSuffix(trimmed, "}") {
		return errors.Errorf("truncated hardware config output (first line %q), the captured PTY stream was "+
			"cut off before the closing brace", firstLine)
	}

	return nil
}
