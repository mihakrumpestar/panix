package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

func (w *WorkflowExecutor) activateMachine(flakeName, configName, machineName string, machine *config.Machine, cm ConfigurationMetadata) error {
	if cm.BuildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path, cannot activate", flakeName, configName, machineName)
	}

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, machine)
	if err != nil {
		return err
	}

	output, err := exc.Exec("sudo", fmt.Sprintf("%s/bin/switch-to-configuration", cm.BuildOutputPath), "switch")
	if err != nil {
		return fmt.Errorf("activation failed for %s/%s/%s: %w\nOutput: %s", flakeName, configName, machineName, err, output.Stderr.String())
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Activated %s/%s/%s successfully\n", flakeName, configName, machineName)
	}

	return nil
}
