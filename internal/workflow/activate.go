package workflow

import (
	"fmt"
	"os/exec"
)

func (w *WorkflowExecutor) executeActivate(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	result := ExecutionResult{
		Machine: machine,
		Phase:   PhaseActivate,
	}

	if machine.FlakeBuildOutputPath == "" {
		result.Error = fmt.Errorf("machine %s has no build output path, cannot activate", machine.Name)
		return result
	}

	sshTarget := machine.Ssh.Host
	//if machine.Ssh.User != "" {
	//	sshTarget = fmt.Sprintf("%s@%s", machine.Ssh.User, sshTarget)
	//}

	// The command to run on the remote machine.
	// This script is part of the system derivation we built and transferred.
	activationCmd := fmt.Sprintf("sudo %s/bin/switch-to-configuration switch", machine.FlakeBuildOutputPath)

	// Using CommandContext to respect the workflow's timeout/cancellation.
	cmd := exec.CommandContext(w.ctx, "ssh", sshTarget, activationCmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("activation failed for %s: %w\nOutput: %s", machine.Name, err, string(output))
		return result
	}

	result.Output = string(output)

	return result
}
