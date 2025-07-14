package workflow

import (
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

func (w *WorkflowExecutor) executeStatus(nextPhases []workflow_definition.WorkflowPhase) error {
	err := w.executeParallelMachines("status", w.checkMachineStatus, nextPhases)
	if err != nil {
		return err
	}

	w.cfg.PrintMachineMetadataStatusTable()
	return nil
}

// CheckHost performs TCP reachability, SSH login, and bootstrap detection
// depth parameter controls how much information to gather
func (w *WorkflowExecutor) checkMachineStatus(flakeName, configName, machineName string, machine *config.Machine) error {
	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, machine)
	if err != nil {
		return err
	}

	machine.Disabled = true // If any error accures, machine will stay disabled from further processing

	// TCP check
	_, err = exc.Ping()
	if err != nil {
		machine.Metadata.Status.Error = err
		return fmt.Errorf("machine %s unreachable: %w", machineName, err)
	}
	machine.Metadata.Status.Reachable = true

	// SSH connect
	output, err := exc.Exec("exit 0")
	if err != nil {
		machine.Metadata.Status.Error = err
		return fmt.Errorf("ssh failed: %w\n%s", err, &output.Stderr)
	}
	machine.Metadata.Status.SSHConnectable = true

	// Run bootstrap detection
	_, err = exc.Exec("test -e /run/current-system")
	if err != nil {
		return nil // just not bootstrapped
	}
	machine.Metadata.Status.Bootstrapped = true

	// Get current generation
	output, err = exc.Exec("nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
	if err != nil {
		machine.Metadata.Status.Error = err
		return err
	}
	machine.Metadata.Status.CurrentGeneration = strings.TrimSpace(output.Stdout.String())

	// Get last deploy time
	output, err = exc.Exec("stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
	if err != nil {
		machine.Metadata.Status.Error = err
		return err
	}
	machine.Metadata.Status.LastDeployTime = strings.TrimSpace(output.Stdout.String())
	machine.Disabled = false

	return nil
}
