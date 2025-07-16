package executioner

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/alitto/pond/v2"
)

func (ex *Executioner) shellStream(name string, args ...string) <-chan ExecutionerOutput {
	ch := make(chan ExecutionerOutput)

	go func() {
		defer close(ch)

		// prepare initial event
		cmd := exec.CommandContext(ex.ctx, name, args...)
		excOut := ExecutionerOutput{
			Command: strings.Join(cmd.Args, " "),
		}
		ch <- excOut

		// dry-run short-circuit
		if ex.dryRun {
			fmt.Println(excOut.Command)
			return
		}

		// wire up pipes
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			excOut.Error = err
			ch <- excOut
			return
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			excOut.Error = err
			ch <- excOut
			return
		}

		// start the process
		if err := cmd.Start(); err != nil {
			excOut.Error = err
			ch <- excOut
			return
		}

		// use a tiny pond pool of size 2 to read both streams
		pool := pond.NewPool(2)
		group := pool.NewGroupContext(ex.ctx)

		group.Submit(func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				toIngest := scanner.Text() + "\n"

				excOut.Stdout.WriteString(toIngest)
				excOut.StdCombined.WriteString(toIngest)
				ch <- excOut
			}
		})
		group.Submit(func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				toIngest := scanner.Text() + "\n"

				excOut.Stderr.WriteString(toIngest)
				excOut.StdCombined.WriteString(toIngest)
				ch <- excOut
			}
		})

		// wait for both readers to finish
		_ = group.Wait()

		// finally wait for the process itself
		if err := cmd.Wait(); err != nil {
			excOut.Error = err
			ch <- excOut
			return
		}

		// one last event on success
		ch <- excOut
	}()

	return ch
}
