package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeBuild(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing build phase\n")
	}

	// Build phase branches on Flakes and Configurations and cascades this branching
	// to all subsequent phases. This is handled by the executeBranching function.

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return &ExecutionResult{}, nil
}

// This function is called by executeMachineBuild for individual machine builds
func (w *WorkflowExecutor) buildFlakeConfiguration(flakeName, configName string, flake config.Flake) error {
	if w.cfg.Global.DryRun {
		fmt.Printf("DRY RUN: Would build %s/%s\n", flakeName, configName)
		return nil
	}

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
	cmd := exec.CommandContext(w.ctx, "nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+ref)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("build failed for %s/%s: %w: %s", flakeName, configName, err, errBuf.String())
	}

	var nr []struct {
		Outputs struct {
			Out string `json:"out"`
		} `json:"outputs"`
	}
	err = json.Unmarshal(outBuf.Bytes(), &nr)
	if err != nil || len(nr) == 0 {
		return fmt.Errorf("invalid build output for %s/%s: %s", flakeName, configName, outBuf.String())
	}

	// Store the build output path in metadata for later phases like transfer and activate
	buildOutputPath := nr[0].Outputs.Out
	w.setBuildOutputPath(flakeName, configName, buildOutputPath)

	if w.cfg.Global.Verbose {
		fmt.Printf("Built %s/%s -> %s\n", flakeName, configName, buildOutputPath)
	}

	return nil
}
