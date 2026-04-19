package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

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

var (
	ErrInvalidBuildOutput = errors.New("invalid build output")
	ErrNoBuildOutputs     = errors.New("invalid build output: no outputs")
)

var nixExperimentalFeatures = []string{"--extra-experimental-features", "nix-command flakes"}

func (w *Workflow) executeBuildPhaseConfiguration(fleetLeaf *fleet.FleetLeaf) error {
	return w.Phase(phase.Build, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error {
			flake := fleetLeaf.Flake
			configurationI := fleetLeaf.Configuration

			if configurationI.MetaBuild == nil {
				configurationI.MetaBuild = &configuration.MetaBuild{}
			}

			flakeOutput := strings.ReplaceAll(configurationI.FlakeOutput.String(), "<name>", configurationI.Name)
			installables := []string{fmt.Sprintf("%s#%s", flake.URL, flakeOutput)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, fleetLeaf, installables, "system closure")
			if err != nil {
				return err
			}

			configurationI.MetaBuild.SystemClosure = parsedOutput[0].Outputs.Out

			return nil
		})
}

func (w *Workflow) executeBuildPhaseConfigurationWrapper(
	exc *executioner.Executioner,
	fleetLeaf *fleet.FleetLeaf,
	installables []string,
	whatIsBuilding string,
) (BuildOutputJSON, error) {
	flake := fleetLeaf.Flake
	configuration := fleetLeaf.Configuration

	var parsedOutput BuildOutputJSON

	commandWithArgs := slices.Concat(
		[]string{"nix"},
		nixExperimentalFeatures,
		[]string{"build", "--no-link", "--no-update-lock-file", "--json"},
		slices.Concat(configuration.Nix.ExtraFlags, configuration.Nix.BuildFlags),
		installables,
	)

	err := exc.Exec(
		"build "+whatIsBuilding,
		"building "+whatIsBuilding,
		whatIsBuilding+" build failed",
		commandWithArgs,
		executioner.DisableAutoSSHCommand(),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			output := lastNonEmptyLine(log.Output.Bytes())

			err := json.Unmarshal(output, &parsedOutput)
			if err != nil || len(parsedOutput) == 0 {
				return errors.Wrapf(ErrInvalidBuildOutput, "%s/%s: %s", flake.Name, configuration.Name, strconv.Quote(string(output)))
			}

			return nil
		}),
		executioner.OnDryRun(func() {
			parsedOutput = BuildOutputJSON{{Outputs: struct {
				Out string `json:"out"`
			}{Out: strcase.SnakeCase(whatIsBuilding) + "_BUILD_OUTPUT_PATH_PLACEHOLDER"}}}
		}),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute build phase")
	}

	if len(parsedOutput) == 0 {
		return nil, errors.Wrapf(ErrNoBuildOutputs, "%s/%s", flake.Name, configuration.Name)
	}

	log.Info().
		Str("flake", flake.Name).
		Str("configuration", configuration.Name).
		Str("closure", configuration.MetaBuild.SystemClosure).
		Msgf("Built %s/%s -> %s", flake.Name, configuration.Name, parsedOutput[0].Outputs.Out)

	return parsedOutput, nil
}

type BuildOutputJSON []struct {
	Outputs struct {
		Out string `json:"out"`
	} `json:"outputs"`
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
