package phaseops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// GenerateHardwareConfig writes the target's nixos-generate-config output to
// machineI.HardwareConfigPath, a path on the machine running panix. No-op when
// that path is empty; the target must already run the installer.
func GenerateHardwareConfig(exc *executioner.Executioner, machineI *machine.Machine) error {
	if machineI.HardwareConfigPath == "" {
		return nil
	}

	err := exc.Exec(
		"generate config",
		"generating hardware config",
		"nixos-generate-config failed",
		append(machineI.MaybeSudo(), "nixos-generate-config", "--show-hardware-config", "--no-filesystems"),
		executioner.OnSuccess(func(commandLog *command.CommandLog) error {
			writeErr := writeHardwareConfig(machineI.HardwareConfigPath, commandLog.Output.String())
			if writeErr != nil {
				return writeErr
			}

			log.Info().
				Str("xpath", machineI.Xpath.String()).
				Str("hardware_config_path", machineI.HardwareConfigPath).
				Msg("wrote hardware config locally")

			stageHardwareConfig(machineI.HardwareConfigPath)

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

// hardwareConfigFromOutput validates captured output before it is written: the
// executioner merges stderr into the same PTY stream, so only a recognized
// nixos-generate-config result may pass. Any mismatch returns an actionable
// error and nothing is written; the content ends with exactly one newline.
func hardwareConfigFromOutput(output string) ([]byte, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, errors.New("nixos-generate-config produced no output, stderr is merged into the captured PTY stream, " +
			"so a warning or error may be all that was captured")
	}

	firstLine, _, _ := strings.Cut(trimmed, "\n")

	// Generated output starts with the "Do not modify this file!" header or
	// the attribute set; anything else is captured stderr noise.
	if !strings.HasPrefix(firstLine, "#") && !strings.HasPrefix(firstLine, "{") {
		return nil, errors.Errorf("unexpected nixos-generate-config output (first line %q), stderr is merged into the "+
			"captured PTY stream, so the target printed a warning or error instead of the config", firstLine)
	}

	if !strings.Contains(trimmed, "modulesPath") || !strings.Contains(trimmed, "config,") {
		return nil, errors.Errorf("unexpected nixos-generate-config output (first line %q), the captured output is not "+
			"a hardware config", firstLine)
	}

	if !strings.HasSuffix(trimmed, "}") {
		return nil, errors.Errorf("truncated nixos-generate-config output (first line %q), the captured PTY stream was "+
			"cut off before the closing brace", firstLine)
	}

	return []byte(trimmed + "\n"), nil
}
