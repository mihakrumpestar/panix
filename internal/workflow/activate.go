package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

// Note: executeActivate is not used - activate phase uses executeActivatePhase in workflow.go
// which handles the special synchronous execution and rollback logic

// This function is called by executeMachineActivate for individual machine activation
func (w *WorkflowExecutor) activateMachine(flakeName, configName, machineName string, machine *config.Machine, buildOutputPath string) error {
	if buildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path, cannot activate", flakeName, configName, machineName)
	}

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, machine)
	if err != nil {
		return err
	}

	output, err := exc.Exec("sudo", fmt.Sprintf("%s/bin/switch-to-configuration", buildOutputPath), "switch")
	if err != nil {
		return fmt.Errorf("activation failed for %s/%s/%s: %w\nOutput: %s", flakeName, configName, machineName, err, output.Stderr.String())
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Activated %s/%s/%s successfully\n", flakeName, configName, machineName)
	}

	return nil
}
