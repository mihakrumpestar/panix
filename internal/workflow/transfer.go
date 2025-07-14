package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

// This function is called by executeMachineTransfer for individual machine transfers
func (w *WorkflowExecutor) transferToMachine(flakeName, configName, machineName string, machine *config.Machine, cm *ConfigurationMetadata) error {
	if cm.BuildOutputPath == "" {
		return fmt.Errorf("machine %s/%s/%s has no build output path", flakeName, configName, machineName)
	}

	exc, err := executioner.New(w.ctx, w.cfg.Global.DryRun, machine)
	if err != nil {
		return err
	}

	sshInfo := fmt.Sprintf("ssh://%s", machine.Ssh.Alias)
	output, err := exc.Exec("nix", "copy", "--to", sshInfo, cm.BuildOutputPath)
	if err != nil {
		return fmt.Errorf("nix copy failed: %w\n%s", err, output.Stderr.String())
	}

	if w.cfg.Global.Verbose {
		fmt.Printf("Transferred %s to %s/%s/%s\n", cm.BuildOutputPath, flakeName, configName, machineName)
	}

	return nil
}
