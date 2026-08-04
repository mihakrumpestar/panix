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
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
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

	commandWithArgs := nixBuildCommand(installable, machine, installables)

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

func nixBuildCommand(installable *installable.Installable, machineI *machine.Machine, installables []string) []string {
	baseArgs := []string{"nix"}
	baseArgs = append(baseArgs, installable.Nix.GetExperimentalFeatures()...)
	baseArgs = append(baseArgs, "build")

	if installable.Nix.BuildMode == nix.BuildModeRemote {
		storeURL := machineI.SSH.NixStoreURL()
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

	sshOpts := machineI.GetActiveSSH().MaybeNixSSHOpts()

	return slices.Concat(evalCacheEnv, sshOpts), nil
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
