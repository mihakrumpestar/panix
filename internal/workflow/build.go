package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// This function is called by executeMachineBuild for individual machine builds
func (w *Workflow) executeBuildPhaseConfiguration(flake *config.Flake, configuration *config.Configuration) error {
	return w.Phase(configuration.Logs.SafeGet(phases.Build),
		fmt.Sprintf("Started build of %s/%s", flake.Name, configuration.Name),
		fmt.Sprintf("Finished build of %s/%s", flake.Name, configuration.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {
			if w.state.Conf.Global.DryRun {
				configuration.MetaBuild.OutputPath = "BUILD_OUTPUT_PATH_PLACEHOLDER"
			}

			// Get the flake path from the flake configuration
			flakePath := flake.Url

			ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", flakePath, configuration.Name)

			// Build a configuration
			err := exc.Exec(false, true, nil,
				func(log *config.CommandLog) error {
					output := log.Bytes()
					output = lastNonEmptyLine(output)

					if w.state.Conf.Global.DryRun {
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

					var parsedOutput BuidOutputJson
					err := json.Unmarshal(output, &parsedOutput)
					if err != nil || len(parsedOutput) == 0 {
						return fmt.Errorf("invalid build output for %s/%s: %s", flake.Name, configuration.Name, strconv.Quote(string(output)))
					}

					configuration.MetaBuild.OutputPath = parsedOutput[0].Outputs.Out

					return nil
				}, // "--print-build-logs"
				"nix", "build", "--no-link", "--no-update-lock-file", "--json", "--cores", "0", "path:"+ref, // The following options don't seem to do anything: "--log-format", "bar-with-logs"
			)
			if err != nil {
				return err
			}

			if w.state.Conf.Global.Verbose {
				phaseLog.AddMessageOnly(fmt.Sprintf("Built %s/%s -> %s\n", flake.Name, configuration.Name, configuration.MetaBuild.OutputPath))
			}

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
