package workflow

import (
	"fmt"
	"os/exec"

	"github.com/mihakrumpestar/panix/internal/config"
)

func (w *WorkflowExecutor) executeTransfer(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	result := ExecutionResult{
		Machine: machine,
		Phase:   PhaseStatus,
	}

	if machine.FlakeBuildOutputPath == "" {
		result.Error = fmt.Errorf("machine %s has no build output path", machine.Name)
		return result
	}

	var err error
	switch machine.Transport {
	case "ssh":
		err = nixStoreCopy(machine, machine.FlakeBuildOutputPath)
	case "tarball":
		err = fmt.Errorf("tarball transport not yet implemented")
	default:
		err = nixStoreCopy(machine, machine.FlakeBuildOutputPath)
	}
	if err != nil {
		result.Error = err
		return result
	}

	result.Output = "Transfer completed"

	return result
}

func nixStoreCopy(machine config.MachineConfig, path string) error {
	sshInfo := fmt.Sprintf("ssh://%s", machine.Ssh.Host)
	cmd := exec.Command("nix", "copy", "--to", sshInfo, path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nix copy failed: %w\n%s", err, string(output))
	}

	return nil
}
