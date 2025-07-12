package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

func (w *WorkflowExecutor) executeBuild(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	result := ExecutionResult{
		Machine: machine,
		Phase:   PhaseBuild,
	}

	abs, err := filepath.Abs(machine.FlakePath)
	if err != nil {
		result.Error = err
		return result
	}

	ref := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.toplevel", abs, machine.FlakeOutput)
	cmd := exec.CommandContext(w.ctx, "nix", "build", "--no-link", "--no-update-lock-file", "--json", "path:"+ref)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		result.Error = fmt.Errorf("%w: %s", err, errBuf.String())
		return result
	}

	var nr []struct {
		Outputs struct {
			Out string `json:"out"`
		} `json:"outputs"`
	}
	err = json.Unmarshal(outBuf.Bytes(), &nr)
	if err != nil || len(nr) == 0 {
		result.Error = fmt.Errorf("invalid build output: %s", outBuf.String())
		return result
	}

	result.Machine.FlakeBuildOutputPath = nr[0].Outputs.Out
	result.Output = "Build OK"

	return result
}
