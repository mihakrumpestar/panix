package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/stoewer/go-strcase"
)

var ErrNoBuildOutputs = errors.New("invalid build output: no outputs")

var NixExperimentalFeatures = []string{"--extra-experimental-features", "nix-command flakes"}

func (w *Workflow) executeBuildPhaseConfiguration(fleetLeaf *fleet.FleetLeaf) error {
	return w.Phase(phase.Build, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error {
			flake := fleetLeaf.Flake
			configurationI := fleetLeaf.Configuration

			if configurationI.MetaBuild == nil {
				configurationI.MetaBuild = &configuration.MetaBuild{}
			}

			flakeOutput := configuration.ResolveFlakeInstallable(configurationI.FlakeOutput, configurationI.BuildPath, configurationI.Name)
			installables := []string{fmt.Sprintf("%s#%s", flake.URL, flakeOutput)}

			storePath, err := w.executeBuildPhaseConfigurationWrapper(exc, fleetLeaf, installables, "system closure")
			if err != nil {
				return err
			}

			configurationI.MetaBuild.SystemClosure = storePath

			return nil
		})
}

func (w *Workflow) executeBuildPhaseConfigurationWrapper(
	exc *executioner.Executioner,
	fleetLeaf *fleet.FleetLeaf,
	installables []string,
	whatIsBuilding string,
) (string, error) {
	configurationI := fleetLeaf.Configuration
	machine := fleetLeaf.Machine

	commandWithArgs := nixBuildCommand(configurationI, machine, installables)

	var storePath string

	env, err := nixBuildEnv(configurationI, machine)
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
		executioner.OnSuccess(func(log *command.CommandLog) error {
			storePath = style.StripANSI(string(log.Output.LastLine()))

			if storePath == "" || !strings.HasPrefix(storePath, "/nix/store/") {
				return errors.Wrapf(ErrNoBuildOutputs, "%s/%s: %s", fleetLeaf.Flake.Name, configurationI.Name, strconv.Quote(storePath))
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
		Str("flake", fleetLeaf.Flake.Name).
		Str("configuration", configurationI.Name).
		Str("closure", storePath).
		Str("mode", configurationI.Nix.BuildMode.String()).
		Msgf("Built %s/%s -> %s", fleetLeaf.Flake.Name, configurationI.Name, storePath)

	return storePath, nil
}

func nixBuildCommand(configurationI *configuration.Configuration, machineI *machine.Machine, installables []string) []string {
	baseArgs := []string{"nix"}
	baseArgs = append(baseArgs, NixExperimentalFeatures...)
	baseArgs = append(baseArgs, "build")

	if configurationI.Nix.BuildMode == nix.BuildModeRemote {
		storeURL := machineI.SSH.NixStoreURL()
		baseArgs = append(baseArgs,
			"--eval-store", "auto", "--store", storeURL, "--option", "builders", "")
	}

	baseArgs = append(baseArgs, "--no-link", "--no-update-lock-file", "--print-out-paths")

	return slices.Concat(
		baseArgs,
		slices.Concat(configurationI.Nix.ExtraFlags, configurationI.Nix.BuildFlags),
		installables,
	)
}

func nixBuildEnv(configurationI *configuration.Configuration, machineI *machine.Machine) ([]string, error) {
	evalCacheEnv, err := nixEvalCacheEnv(configurationI.Xpath.String())
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
