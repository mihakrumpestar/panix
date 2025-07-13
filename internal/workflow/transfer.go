package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeTransfer(nextPhases []workflow_definition.WorkflowPhase) error {
	return w.executeParallelMachines("transfer", w.transferToMachine, nextPhases)
}

// This function is called by executeMachineTransfer for individual machine transfers
func (w *WorkflowExecutor) transferToMachine(flakeName, configName, machineName string, machine *config.Machine) error {
	buildOutputPath := w.cfg.Flakes[flakeName].Configurations[configName].GetBuildOutputPath()
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

	if w.cfg.Global.Verbose {
		fmt.Printf("Transferred %s to %s/%s/%s\n", buildOutputPath, flakeName, configName, machineName)
	}

	return nil
}
