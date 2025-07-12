package executioner

import (
	"fmt"
	"os/exec"
	"strings"
)

func (ex *Executioner) shell(name string, args ...string) (ExecutionerOutput, error) {

	cmd := exec.CommandContext(ex.ctx, name, args...)
	output := ExecutionerOutput{
		Command: strings.Join(cmd.Args, " "),
	}

	fmt.Println(output.Command)

	cmd.Stdout = &output.Stdout
	cmd.Stderr = &output.Stderr
	err := cmd.Run()
	if err != nil {
		return output, err
	}

	return output, nil
}
