package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/filepermissions"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
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
	flake := fleetLeaf.Flake
	configuration := fleetLeaf.Configuration

	var storePath string

	commandWithArgs := slices.Concat(
		[]string{"nix"},
		NixExperimentalFeatures,
		[]string{"build", "--no-link", "--no-update-lock-file", "--print-out-paths"},
		slices.Concat(configuration.Nix.ExtraFlags, configuration.Nix.BuildFlags),
		installables,
	)

	// Isolate the eval cache per configuration to avoid SQLite busy warnings
	// when building multiple configurations in parallel.
	env, err := nixEvalCacheEnv(configuration.Xpath.String())
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
			storePath = strings.TrimSpace(string(lastNonEmptyLine(log.Output.Bytes())))

			if storePath == "" || !strings.HasPrefix(storePath, "/nix/store/") {
				return errors.Wrapf(ErrNoBuildOutputs, "%s/%s: %s", flake.Name, configuration.Name, strconv.Quote(storePath))
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
		Str("flake", flake.Name).
		Str("configuration", configuration.Name).
		Str("closure", storePath).
		Msgf("Built %s/%s -> %s", flake.Name, configuration.Name, storePath)

	return storePath, nil
}

func lastNonEmptyLine(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	slices.Reverse(lines)

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			return trimmed
		}
	}

	return []byte{}
}

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
