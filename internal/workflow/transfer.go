package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeTransfer(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if w.cfg.Global.Verbose {
		fmt.Printf("Executing transfer phase\n")
	}

	// Transfer phase should already be fully branched from previous phases
	// (either from Build or Bootstrap phase branching)

	// Execute the transfer phase with branching
	_, err := w.executeBranching(workflow_definition.PhaseTransfer, []workflow_definition.WorkflowPhase{}, true, true, true)
	if err != nil {
		return w.metadata, err
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return w.metadata, nil
}

// This function is called by executeMachineTransfer for individual machine transfers
func (w *WorkflowExecutor) transferToMachine(flakeName, configName, machineName string, machine *config.Machine, buildOutputPath string) error {
	if buildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path", flakeName, configName, machineName)
	}

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, machine)
	if err != nil {
		return err
	}

	sshInfo := fmt.Sprintf("ssh://%s", machine.Ssh.Alias)
	output, err := exc.Exec("nix", "copy", "--to", sshInfo, buildOutputPath)
	if err != nil {
		return fmt.Errorf("nix copy failed: %w\n%s", err, output.Stderr.String())
	}

	if err != nil {
		return fmt.Errorf("transfer failed for %s/%s/%s: %w", flakeName, configName, machineName, err)
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Transferred %s to %s/%s/%s\n", buildOutputPath, flakeName, configName, machineName)
	}

	return nil
}
