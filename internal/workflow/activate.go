package workflow

import (
	"fmt"
	"os/exec"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeActivate(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing activate phase\n")
	}

	// Activate phase is special - it should already be fully branched from previous phases
	// but executes synchronously across all machines with rollback capability.
	// This is handled by the executeActivatePhase function in workflow.go

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return &ExecutionResult{}, nil
}

// This function is called by executeMachineActivate for individual machine activation
func (w *WorkflowExecutor) activateMachine(flakeName, configName, machineName string, machine config.Machine, buildOutputPath string) error {
	if w.cfg.Global.DryRun {
		fmt.Printf("DRY RUN: Would activate %s on %s/%s/%s\n", buildOutputPath, flakeName, configName, machineName)
		return nil
	}

	if buildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path, cannot activate", flakeName, configName, machineName)
	}

	sshTarget := machine.Ssh.Host
	if machine.Ssh.User != "" {
		sshTarget = fmt.Sprintf("%s@%s", machine.Ssh.User, sshTarget)
	}

	// The command to run on the remote machine.
	// This script is part of the system derivation we built and transferred.
	activationCmd := fmt.Sprintf("sudo %s/bin/switch-to-configuration switch", buildOutputPath)

	// Using CommandContext to respect the workflow's timeout/cancellation.
	cmd := exec.CommandContext(w.ctx, "ssh", sshTarget, activationCmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("activation failed for %s/%s/%s: %w\nOutput: %s", flakeName, configName, machineName, err, string(output))
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Activated %s/%s/%s successfully\n", flakeName, configName, machineName)
	}

	return nil
}
