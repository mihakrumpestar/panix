package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// This function is called by executeMachineBuild for individual machine builds
func (w *Workflow) executeBuildPhaseConfiguration(flake *config.Flake, configuration *config.Configuration) error {
	return w.Phase(configuration.Attributes.Xpath, phases.Build, nil,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {
			if w.state.Conf.Flags.DryRun {
				configuration.MetaBuild.SystemClosure = "BUILD_OUTPUT_PATH_PLACEHOLDER"
			}

			mb := configuration.MetaBuild

			// System closure
			installables := []string{fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", flake.Url, configuration.Name)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, phaseLog, flake, configuration, installables)
			if err != nil {
				return err
			}

			mb.SystemClosure = parsedOutput[0].Outputs.Out

			return nil
		})
}

func (w *Workflow) executeBuildPhaseConfigurationWrapper(exc *executioner.Executioner, phaseLog *logs.PhaseLog, flake *config.Flake, configuration *config.Configuration, installables []string) (BuidOutputJson, error) {
	var parsedOutput BuidOutputJson

	commandWithArgs := append([]string{"nix", "build", "--no-link", "--no-update-lock-file", "--json"}, installables...)

	err := exc.Exec(
		"build",
		"build failed",
		commandWithArgs,
		executioner.DisableAutoSshCommand(),
		executioner.OnSuccess(func(log *logs.CommandLog) error {
			output := log.Bytes()
			output = lastNonEmptyLine(output)

			if w.state.Conf.Flags.DryRun {
				output = []byte(`
						[
							{
								"outputs": {
									"out": "DRY_RUN"
								}
							}
						]
						`)
			}

			err := json.Unmarshal(output, &parsedOutput)
			if err != nil || len(parsedOutput) == 0 {
				return fmt.Errorf("invalid build output for %s/%s: %s", flake.Name, configuration.Name, strconv.Quote(string(output)))
			}

			return nil
		}), // "--print-build-logs"
	)
	if err != nil {
		return nil, err
	}

	phaseLog.Verbose("Built %s/%s -> %s\n", flake.Name, configuration.Name, configuration.MetaBuild.SystemClosure)

	return parsedOutput, nil
}

// Unmarshall
type BuidOutputJson []struct {
	Outputs struct {
		Out string `json:"out"`
	} `json:"outputs"`
}

// Helpers

func lastNonEmptyLine(b []byte) []byte {
	// Split on newlines
	lines := bytes.Split(b, []byte("\n"))

	// From last
	slices.Reverse(lines)

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)

		if len(trimmed) > 0 {
			return trimmed
		}
	}

	return []byte{}
}
