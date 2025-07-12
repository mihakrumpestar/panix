package workflow

import (
	"fmt"
	"os/exec"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeTransfer(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing transfer phase\n")
	}

	// Transfer phase should already be fully branched from previous phases
	// (either from Build or Bootstrap phase branching)

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return &ExecutionResult{}, nil
}

// This function is called by executeMachineTransfer for individual machine transfers
func (w *WorkflowExecutor) transferToMachine(flakeName, configName, machineName string, machine config.Machine, buildOutputPath string) error {
	if w.cfg.Global.DryRun {
		fmt.Printf("DRY RUN: Would transfer %s to %s/%s/%s\n", buildOutputPath, flakeName, configName, machineName)
		return nil
	}

	if buildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path", flakeName, configName, machineName)
	}

	// Determine transport method
	transport := "ssh" // default
	// TODO: Add transport configuration to machine config if needed

	var err error
	switch transport {
	case "ssh":
		err = w.nixStoreCopy(machine, buildOutputPath)
	case "tarball":
		err = fmt.Errorf("tarball transport not yet implemented")
	default:
		err = w.nixStoreCopy(machine, buildOutputPath)
	}

	if err != nil {
		return fmt.Errorf("transfer failed for %s/%s/%s: %w", flakeName, configName, machineName, err)
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Transferred %s to %s/%s/%s\n", buildOutputPath, flakeName, configName, machineName)
	}

	return nil
}

func (w *WorkflowExecutor) nixStoreCopy(machine config.Machine, path string) error {
	sshInfo := fmt.Sprintf("ssh://%s", machine.Ssh.Host)
	cmd := exec.CommandContext(w.ctx, "nix", "copy", "--to", sshInfo, path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nix copy failed: %w\n%s", err, string(output))
	}

	return nil
}
