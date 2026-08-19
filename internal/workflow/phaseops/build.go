package phaseops

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/stoewer/go-strcase"
)

var ErrNoBuildOutputs = errors.New("invalid build output: no outputs")

func BuildInstallable(
	exc *executioner.Executioner,
	fleetLeaf *fleet.FleetLeaf,
	installables []string,
	whatIsBuilding string,
) (string, error) {
	installable := fleetLeaf.Installable
	machine := fleetLeaf.Machine

	commandWithArgs := nixBuildCommand(installable, installables)

	var storePath string

	env, err := nixBuildEnv(installable, machine)
	if err != nil {
		return "", err
	}

	err = exc.Exec(
		"build "+whatIsBuilding,
		"building "+whatIsBuilding,
		whatIsBuilding+" build failed",
		commandWithArgs,
		executioner.DisableAutoSSHCommand(),
		executioner.Env(env),
		executioner.Trim(),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			storePath = string(style.StripANSI(log.Output.LastLine()))

			if storePath == "" || !strings.HasPrefix(storePath, "/nix/store/") {
				return errors.Wrapf(ErrNoBuildOutputs, "%s/%s: %s", fleetLeaf.Flake.Name, installable.Name, strconv.Quote(storePath))
			}

			return nil
		}),
		executioner.OnDryRun(func() {
			storePath = strcase.SnakeCase(whatIsBuilding) + "_BUILD_OUTPUT_PATH_PLACEHOLDER"
		}),
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute build phase")
	}

	log.Info().
		Str("flake", fleetLeaf.Flake.Name.String()).
		Str("installable", installable.Name.String()).
		Str("closure", storePath).
		Str("mode", installable.Nix.BuildMode.String()).
		Msgf("Built %s/%s -> %s", fleetLeaf.Flake.Name, installable.Name, storePath)

	return storePath, nil
}

// remoteBuilderSSH returns the SSH endpoint remote-mode commands target:
// the installable's pinned builder (first declared machine, see
// Installable.RemoteBuilder). Panics on the validation-excluded no-machines
// case, mirroring Machine.GetActiveSSH's internal-error panic; onceasync and
// the worker pool recover panics into phase errors.
func remoteBuilderSSH(installable *installable.Installable) ssh.SSHClient {
	builder := installable.RemoteBuilder()
	if builder == nil {
		panic("internal error: remote-mode installable has no machines (validation should have rejected it)")
	}

	return builder.GetActiveSSH()
}

func nixBuildCommand(installable *installable.Installable, installables []string) []string {
	baseArgs := []string{"nix"}
	baseArgs = append(baseArgs, installable.Nix.GetExperimentalFeatures()...)
	baseArgs = append(baseArgs, "build")

	if installable.Nix.BuildMode == nix.BuildModeRemote {
		// The builder is pinned to the first declared machine, so the command
		// is identical regardless of which machine's goroutine executes it.
		storeURL := remoteBuilderSSH(installable).NixStoreURL()
		baseArgs = append(baseArgs,
			"--eval-store", "auto", "--store", storeURL, "--option", "builders", "")
	}

	baseArgs = append(baseArgs, installable.Nix.GetBuildDefaultFlags()...)

	return slices.Concat(
		baseArgs,
		slices.Concat(installable.Nix.ExtraFlags, installable.Nix.BuildFlags),
		installables,
	)
}

func nixBuildEnv(installable *installable.Installable, machineI *machine.Machine) ([]string, error) {
	evalCacheEnv, err := nixEvalCacheEnv(installable.Xpath.String())
	if err != nil {
		return nil, err
	}

	// In remote mode, NIX_SSHOPTS must target the pinned builder machine
	// (the same machine as the --store URL), not whichever machine's
	// goroutine executed the build.
	builderSSH := machineI.GetActiveSSH()
	if installable.Nix.BuildMode == nix.BuildModeRemote {
		builderSSH = remoteBuilderSSH(installable)
	}

	sshOpts := builderSSH.MaybeNixSSHOpts()

	// User env first so panix-internal vars (XDG_CACHE_HOME, NIX_SSHOPTS)
	// take precedence on key conflicts.
	return slices.Concat(installable.Nix.GetBuildEnv(), evalCacheEnv, sshOpts), nil
}

// nixEvalCacheEnv isolates the eval cache per configuration to avoid SQLite busy warnings
// when building multiple configurations in parallel.
func nixEvalCacheEnv(xpath string) ([]string, error) {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user cache dir")
	}

	evalCacheDir := filepath.Join(userCacheDir, "panix", xpath)

	err = os.MkdirAll(evalCacheDir, filepermissions.DefaultDirPermissions)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create eval cache dir")
	}

	return []string{"XDG_CACHE_HOME=" + evalCacheDir}, nil
}
