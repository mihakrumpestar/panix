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

// executeBuildPhase runs builds in parallel across configurations
// As soon as a configuration succeeds, applicable machines proceed with bootstrap/transfer/secrets
func (w *Workflow) ExecuteBuildPhase() error {
	return poolChildren(w, w.state.Conf.Root, true, func(flake *config.Flake) error {
		return w.Phase(flake.Logs.SafeGet(phases.PreFlakeHook),
			fmt.Sprintf("Started build across %s configurations", flake.Name),
			fmt.Sprintf("Finished build across %s configurations", flake.Name),
			nil,
			func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {

				if flake.BuildHooks.Pre != "" {
					// Pre build hook
					err := exc.Exec(false, true, nil, nil,
						"sh", "-c", flake.BuildHooks.Pre,
					)
					if err != nil {
						return err
					}
				}

				return poolChildren(w, flake, true, func(configuration *config.Configuration) error {
					return w.Phase(flake.Logs.SafeGet(phases.Build),
						fmt.Sprintf("Started build on %s configuration", configuration.Name),
						fmt.Sprintf("Finished build on %s configuration", configuration.Name),
						nil,
						func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {
							err := w.executeBuildPhaseConfiguration(flake, configuration)
							if err != nil {
								return err
							}

							if w.state.Conf.Global.Verbose {
								phaseLog.AddMessageOnly("Executing finished for build phase ", flake.Name)
							}

							return poolChildren(w, configuration, true, func(machine *config.Machine) error {
								if slices.Contains(w.phases, phases.Transfer) {
									err := w.executeTransferPhaseMachine(configuration, machine)
									if err != nil {
										return err
									}
								}

								if slices.Contains(w.phases, phases.Secrets) {
									err := w.executeSecretsPhaseMachine(machine)
									if err != nil {
										return err
									}
								}

								if slices.Contains(w.phases, phases.Activate) {
									err = w.executeActivatePhaseMachine(configuration, machine)
									if err != nil {
										return err
									}
								}

								return nil
							})
						})
				})
			})
	})
}

type BuidOutputJson struct {
	Outputs struct {
		Out string `json:"out"`
	} `json:"outputs"`
}

// This function is called by executeMachineBuild for individual machine builds
func (w *Workflow) executeBuildPhaseConfiguration(flake *config.Flake, configuration *config.Configuration) error {
	return w.Phase(configuration.Logs.SafeGet(phases.Build),
		fmt.Sprintf("Started build of %s", configuration.Name),
		fmt.Sprintf("Built %s/%s -> %s\n", flake.Name, configuration.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {

			bm := configuration.MetaBuild

			if w.state.Conf.Global.DryRun {
				bm.OutputPath = "BUILD_OUTPUT_PATH_PLACEHOLDER"
			}

			// Get the flake path from the flake configuration
			flakePath := flake.Url

			ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", flakePath, configuration.Name)

			// Build a configuration
			err := exc.Exec(false, true, nil,
				func(log *config.CommandLog) error {
					output := log.Bytes()
					output = lastNonEmptyLine(output)

					var parsedOutput []BuidOutputJson
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
				phaseLog.AddMessageOnly(fmt.Sprintf("Built %s/%s -> %s\n", flake.Name, configuration.Name, bm.OutputPath))
			}

			return nil
		})
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
