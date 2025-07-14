package workflow

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

type ConfigurationMetadata struct {
	MetadataID
	BuildOutputPath string
}

// This function is called by executeMachineBuild for individual machine builds
func (w *WorkflowExecutor) executeBuildPhase(flakeName, configurationName string, flake *config.Flake) (cm *ConfigurationMetadata, err error) {
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

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, &config.Machine{Local: true})
	if err != nil {
		return
	}
	output, err := exc.Exec("nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+ref)
	if err != nil {
		err = fmt.Errorf("build failed for %s/%s: %w: %s", flakeName, configurationName, err, output.Stderr.String())
		return
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
			err = fmt.Errorf("invalid build output for %s/%s: %s", flakeName, configurationName, output.Stdout.Bytes())
			return
		}
		// Store the build output path in metadata for later phases like transfer and activate
		buildOutputPath = nr[0].Outputs.Out
	}

	cm.BuildOutputPath = buildOutputPath

	if w.cfg.Global.Verbose {
		fmt.Printf("Built %s/%s -> %s\n", flakeName, configurationName, buildOutputPath)
	}

	return
}
