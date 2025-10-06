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
	return w.Phase(&configuration.Attributes, phases.Build, nil,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {
			if w.state.Conf.Flags.DryRun {
				configuration.MetaBuild.SystemClosure = "BUILD_OUTPUT_PATH_PLACEHOLDER"
				configuration.MetaBuild.DiskoScript = "BUILD_OUTPUT_PATH_PLACEHOLDER"
			}

			mb := configuration.MetaBuild

			// System closure
			installables := []string{fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", flake.Url, configuration.Name)}

			// DiskoScript
			if !w.state.Conf.Flags.Bootstrap.DisableAuto {
				installables = append(installables, fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.diskoScript", flake.Url, configuration.Name))
			}

			commandWithArgs := append([]string{"nix", "build", "--no-link", "--no-update-lock-file", "--json"}, installables...)

			err := exc.Exec(false, true, nil,
				func(log *logs.CommandLog) error {
					output := log.Bytes()
					output = lastNonEmptyLine(output)

					if w.state.Conf.Flags.DryRun {
						output = []byte(`
						[
							{
								"outputs": {
									"out": "DRY_RUN"
								}
							},
							{
								"outputs": {
									"out": "DRY_RUN"
								}
							}
						]
						`)
					}

					var parsedOutput BuidOutputJson
					err := json.Unmarshal(output, &parsedOutput)
					if err != nil || len(parsedOutput) == 0 {
						return fmt.Errorf("invalid build output for %s/%s: %s", flake.Name, configuration.Name, strconv.Quote(string(output)))
					}

					mb.SystemClosure = parsedOutput[0].Outputs.Out

					if !w.state.Conf.Flags.Bootstrap.DisableAuto {
						mb.DiskoScript = parsedOutput[1].Outputs.Out
					}

					return nil
				}, // "--print-build-logs"
				commandWithArgs...,
			)
			if err != nil {
				return err
			}

			phaseLog.Verbose("Built %s/%s -> %s\n", flake.Name, configuration.Name, configuration.MetaBuild.SystemClosure)

			return nil
		})
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
