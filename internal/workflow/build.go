package workflow

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

type ConfigurationMetadata struct {
	MetadataID
	BuildOutputPath string
	Error           error
}

// executeBuildPhase runs builds in parallel across configurations
// As soon as a configuration succeeds, applicable machines proceed with bootstrap/transfer/secrets
func (w *WorkflowExecutor) ExecuteBuildPhase(sms []StatusMetadata) ([]ConfigurationMetadata, error) {
	if w.cfg.Global.Verbose {
		fmt.Println("Executing build phase across flake configurations")
	}

	pool := pond.NewResultPool[ConfigurationMetadata](w.cfg.Global.Concurrency)
	buildGroup := pool.NewGroupContext(w.ctx)
	bootstrapGroup := pool.NewGroupContext(w.ctx)
	//transferGroup := pool.NewGroupContext(w.ctx)
	//secretsGroup := pool.NewGroupContext(w.ctx)

	forAllConfigurations(w.cfg.Flakes, sms, func(flakeName, configurationName string, flake *config.Flake, configuration *config.Configuration) {
		buildGroup.SubmitErr(func() (ConfigurationMetadata, error) {
			wp := WorkflowExecutorForConfigurationAndMachine{w.ctx, &w.cfg.Global}
			result, err := wp.executeBuildPhaseConfiguration(flakeName, configurationName, flake)
			result.Error = err

			if result.Error != nil && w.cfg.Global.RequireAllSuccess {
				return result, err
			}

			for machineName, machine := range configuration.Machines {
				if !slices.Contains(w.cfg.Global.SkipPhases, workflow_definition.PhaseBootstrap) {
					bootstrapGroup.SubmitErr(func() (ConfigurationMetadata, error) {

						err := wp.executeBootstrapPhaseMachine(flakeName, configurationName, machineName, machine)

						if err != nil && w.cfg.Global.RequireAllSuccess {
							return result, err
						}

						return result, nil
					})

				}
			}

			return result, nil
		})
	})

	results, _ := buildGroup.Wait()

	errors := make([]error, 0)
	for _, result := range results {
		if result.Error != nil {
			errors = append(errors, result.Error)
		}
	}

	if len(errors) > 0 && w.cfg.Global.RequireAllSuccess {
		return nil, fmt.Errorf("status phase failed: %v", errors)
	}

	//if !slices.Contains(w.cfg.Global.SkipPhases, workflow_definition.PhaseActivate) {
	//	return w.executeActivationPhase(nextPhases)
	//}

	return results, nil
}

// This function is called by executeMachineBuild for individual machine builds
func (w *WorkflowExecutorForConfigurationAndMachine) executeBuildPhaseConfiguration(flakeName, configurationName string, flake *config.Flake) (cm ConfigurationMetadata, err error) {
	cm.MetadataID = MetadataID{
		FlakeName:         flakeName,
		ConfigurationName: configurationName,
	}

	// Get the flake path from the flake configuration
	flakePath := flake.Url
	if flakePath == "" {
		fmt.Errorf("flake %s has no URL configured", flakeName)
		return
	}

	abs, err := filepath.Abs(flakePath)
	if err != nil {
		err = fmt.Errorf("failed to get absolute path for flake %s: %w", flakeName, err)
		return
	}

	ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", abs, configurationName)

	exc, err := executioner.New(w.ctx, w.cfg.DryRun, &config.Machine{Local: true})
	if err != nil {
		return
	}
	output, err := exc.Exec("nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+ref)
	if err != nil {
		err = fmt.Errorf("build failed for %s/%s: %w: %s", flakeName, configurationName, err, output.Stderr.String())
		return
	}

	buildOutputPath := "BUILD_OUTPUT_PATH_PLACEHOLDER"
	if !w.cfg.DryRun {
		var nr []struct {
			Outputs struct {
				Out string `json:"out"`
			} `json:"outputs"`
		}
		err = json.Unmarshal(output.Stdout.Bytes(), &nr)
		if err != nil || len(nr) == 0 {
			err = fmt.Errorf("invalid build output for %s/%s: %s", flakeName, configurationName, output.Stdout.Bytes())
			return
		}
		// Store the build output path in metadata for later phases like transfer and activate
		buildOutputPath = nr[0].Outputs.Out
	}

	cm.BuildOutputPath = buildOutputPath

	if w.cfg.Verbose {
		fmt.Printf("Built %s/%s -> %s\n", flakeName, configurationName, buildOutputPath)
	}

	return
}
