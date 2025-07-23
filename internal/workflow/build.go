package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

// executeBuildPhase runs builds in parallel across configurations
// As soon as a configuration succeeds, applicable machines proceed with bootstrap/transfer/secrets
func (w *Workflow) ExecuteBuildPhase() error {
	err := w.forEachFlakeConfiguration(
		func(flakeName string, configurationName string, flake *config.Flake, configuration *config.Configuration) error {
			log := configuration.Logs.SafeGet(workflow_definition.PhaseBuild)

			if w.state.Conf.Global.Verbose {
				log.AddMessageOnly("Executing build phase across " + configurationName)
			}

			err := w.executeBuildPhaseConfiguration(flakeName, configurationName, flake, configuration)
			if err != nil {
				return err
			}

			if w.state.Conf.Global.Verbose {
				log.AddMessageOnly("Executing finished for build phase ", configurationName)
			}

			err = w.forEachConfigurationMachine(configuration,
				func(machineName url.URL, machine *config.Machine) error {

					if slices.Contains(w.phases, workflow_definition.PhaseTransfer) {
						err := w.executeTransferPhaseMachine(configuration, machineName, machine)
						if err != nil {
							return err
						}
					}

					if slices.Contains(w.phases, workflow_definition.PhaseActivate) {
						err = w.executeActivatePhaseMachine(configuration, machineName, machine)
						if err != nil {
							return err
						}
					}

					return nil
				})

			if err != nil {
				return err
			}

			return err
		})

	if err != nil {
		return err
	}

	return nil
}

type BuidOutputJson struct {
	Outputs struct {
		Out string `json:"out"`
	} `json:"outputs"`
}

// This function is called by executeMachineBuild for individual machine builds
func (w *Workflow) executeBuildPhaseConfiguration(flakeName, configurationName string, flake *config.Flake, configuration *config.Configuration) (err error) {
	log := configuration.Logs.SafeGet(workflow_definition.PhaseBuild)
	log.TimeAndState.StartTimer()
	defer func() {
		log.TimeAndState.EndTimerWithError(err)
	}()

	bm := configuration.Phases.Build

	if w.state.Conf.Global.DryRun {
		bm.BuildOutputPath = "BUILD_OUTPUT_PATH_PLACEHOLDER"
		return
	}

	// Get the flake path from the flake configuration
	flakePath := flake.Url

	abs, err := filepath.Abs(flakePath)
	if err != nil {
		err = fmt.Errorf("failed to get absolute path for flake %s: %w", flakeName, err)
		return
	}

	ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", abs, configurationName)
	//fmt.Sprint(ref)

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, nil, nil, log, w.hook.OnUpdateHook)

	// Build a configuration
	err = exc.Exec(false,
		func(log *config.Log, err error) error {
			return fmt.Errorf("build failed for %s/%s: %w", flakeName, configurationName, err)
		},
		func(log *config.Log) error {
			output := log.LastCommand().StdInOutErr.Bytes()
			output = lastNonEmptyLineWithoutAnsi(output)

			var parsedOutput []BuidOutputJson
			err = json.Unmarshal(output, &parsedOutput)
			if err != nil || len(parsedOutput) == 0 {
				return fmt.Errorf("invalid build output for %s/%s: %s", flakeName, configurationName, strconv.Quote(string(output)))
			}

			configuration.Phases.Build.BuildOutputPath = parsedOutput[0].Outputs.Out

			return nil
		}, // "--print-build-logs"
		"nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+ref, // The following options don't seem to do anything: "--log-format", "bar-with-logs"
	)
	if err != nil {
		return
	}

	if w.state.Conf.Global.Verbose {
		log.AddMessageOnly(fmt.Sprintf("Built %s/%s -> %s\n", flakeName, configurationName, bm.BuildOutputPath))
	}

	return
}

// Helpers

func lastNonEmptyLineWithoutAnsi(b []byte) []byte {

	cleaned := executioner.CleanAnsiAndSpace(b)

	// Split on newlines
	lines := bytes.Split(cleaned, []byte("\n"))

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
