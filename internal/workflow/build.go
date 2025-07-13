package workflow

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeBuild(nextPhases []workflow_definition.WorkflowPhase) error {
	return w.executeParallelConfigs("build", w.buildFlakeConfiguration, nextPhases)
}

// This function is called by executeMachineBuild for individual machine builds
func (w *WorkflowExecutor) buildFlakeConfiguration(flakeName, configName string, flake *config.Flake) error {
	// Get the flake path from the flake configuration
	flakePath := flake.Url
	if flakePath == "" {
		return fmt.Errorf("flake %s has no URL configured", flakeName)
	}

	abs, err := filepath.Abs(flakePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for flake %s: %w", flakeName, err)
	}

	ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", abs, configName)

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, &config.Machine{Local: true})
	if err != nil {
		return err
	}
	output, err := exc.Exec("nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+ref)
	if err != nil {
		return fmt.Errorf("build failed for %s/%s: %w: %s", flakeName, configName, err, output.Stderr.String())
	}

	buildOutputPath := "BUILD_OUTPUT_PATH_PLACEHOLDER"
	if !w.cfg.Global.DryRun {
		var nr []struct {
			Outputs struct {
				Out string `json:"out"`
			} `json:"outputs"`
		}
		err = json.Unmarshal(output.Stdout.Bytes(), &nr)
		if err != nil || len(nr) == 0 {
			return fmt.Errorf("invalid build output for %s/%s: %s", flakeName, configName, output.Stdout.Bytes())
		}
		// Store the build output path in metadata for later phases like transfer and activate
		buildOutputPath = nr[0].Outputs.Out
	}

	w.cfg.Flakes[flakeName].Configurations[configName].SetBuildOutputPath(buildOutputPath)

	if w.cfg.Global.Verbose {
		fmt.Printf("Built %s/%s -> %s\n", flakeName, configName, buildOutputPath)
	}

	return nil
}
