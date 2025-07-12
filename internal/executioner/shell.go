package executioner

import (
	"os/exec"
)

func (ex *Executioner) Shell(name string, args ...string) (ExecutionerOutput, error) {
	output := ExecutionerOutput{}

	cmd := exec.CommandContext(ex.ctx, name, args...)
	cmd.Stdout = &output.Stdout
	cmd.Stderr = &output.Stderr

	err := cmd.Run()
	if err != nil {
		return output, err
	}

	return output, nil
}
