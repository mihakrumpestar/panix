package workflow

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

// executeBuildPhase runs builds in parallel across configurations
// As soon as a configuration succeeds, applicable machines proceed with bootstrap/transfer/secrets
func (w *Workflow) ExecuteBuildPhase() error {
	if w.state.Conf.Global.Verbose {
		fmt.Println("Executing build phase across flake configurations")
	}

	err := w.forEachFlakeConfiguration(func(groupPool pond.TaskGroup, flakeName string, configurationName string, flake *config.Flake, configuration *config.Configuration) error {
		if w.state.Conf.Global.Verbose {
			fmt.Println("Executing status phase across all machines in " + flakeName + " " + configurationName)
		}

		configurationBuildTimeAndState := configuration.Logs[workflow_definition.PhaseBuild].TimeAndState

		configurationBuildTimeAndState.StartTimer()
		err := w.executeBuildPhaseConfiguration(flakeName, configurationName, flake, configuration)
		configurationBuildTimeAndState.EndTimerWithError(err)

		return err
	})

	if w.state.Conf.Global.Verbose {
		fmt.Println("Executing finished for status phase with err %w", err)
	}

	w.state.Error = err
	w.hook.OnUpdateHook()

	return nil
}

type BuidOutputJson struct {
	Outputs struct {
		Out string `json:"out"`
	} `json:"outputs"`
}

// This function is called by executeMachineBuild for individual machine builds
func (w *Workflow) executeBuildPhaseConfiguration(flakeName, configurationName string, flake *config.Flake, configuration *config.Configuration) error {
	bm := configuration.Phases.Build

	if w.state.Conf.Global.DryRun {
		bm.BuildOutputPath = "BUILD_OUTPUT_PATH_PLACEHOLDER"
		return nil
	}

	// Get the flake path from the flake configuration
	flakePath := flake.Url

	abs, err := filepath.Abs(flakePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for flake %s: %w", flakeName, err)
	}

	ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", abs, configurationName)

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, nil, nil, configuration.Logs[workflow_definition.PhaseBuild], w.hook.OnUpdateHook)

	// Build a configuration
	err = exc.Exec(
		func(log *config.Log, err error) error {
			return fmt.Errorf("build failed for %s/%s: %w: %s", flakeName, configurationName, err, log.LastCommand().StdCombined.String())
		},
		func(log *config.Log) error {
			var parsedOutput []BuidOutputJson

			output := []byte(log.LastCommand().Stdout.String())

			err = json.Unmarshal(output, &parsedOutput)
			if err != nil || len(parsedOutput) == 0 {
				return fmt.Errorf("invalid build output for %s/%s: %s", flakeName, configurationName, log.LastCommand().StdCombined.String())
			}

			configuration.Phases.Build.BuildOutputPath = parsedOutput[0].Outputs.Out
			return nil
		},
		"nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+ref,
	)
	if err != nil {
		return err
	}

	if w.state.Conf.Global.Verbose {
		fmt.Printf("Built %s/%s -> %s\n", flakeName, configurationName, bm.BuildOutputPath)
	}

	return nil
}
