package executioner

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alitto/pond/v2"
)

func (ex *Executioner) shellStream(onFailure func(*BaseMeta, error) error, onSuccess func(*BaseMeta), name string, args ...string) error {
	if ex.meta.CommandOutputs == nil {
		ex.meta.CommandOutputs = make([]*ExecutionerMetadata, 0)
	}

	exm := &ExecutionerMetadata{}
	ex.meta.CommandOutputs = append(ex.meta.CommandOutputs, exm)

	// prepare initial event
	cmd := exec.CommandContext(ex.ctx, name, args...)
	exm.Command = strings.Join(cmd.Args, " ")
	ex.onUpdateHook()

	// dry-run short-circuit
	if ex.dryRun {
		fmt.Println(exm.Command)
		if onSuccess != nil {
			onSuccess(ex.meta)
		}
		return nil
	}

	execStream := func() error {
		// wire up pipes
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return err
		}

		// start the process
		err = cmd.Start()
		if err != nil {
			return err
		}

		// use a tiny pond pool of size 2 to read both streams and update hook
		pool := pond.NewPool(2)
		group := pool.NewGroupContext(ex.ctx)

		group.Submit(func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				toIngest := scanner.Text() + "\n"

				exm.Stdout.WriteString(toIngest)
				exm.StdCombined.WriteString(toIngest)
				ex.onUpdateHook()
			}
		})
		group.Submit(func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				toIngest := scanner.Text() + "\n"

				exm.Stderr.WriteString(toIngest)
				exm.StdCombined.WriteString(toIngest)
				ex.onUpdateHook()
			}
		})

		// wait for both readers to finish
		_ = group.Wait()

		// finally wait for the process itself
		return cmd.Wait()
	}

	// Blocking stream with real time updates
	exm.StartTime = time.Now()
	ex.onUpdateHook()

	exm.Error = execStream()
	exm.EndTime = time.Now()

	if exm.Error != nil {
		if onFailure != nil {
			ex.meta.Error = onFailure(ex.meta, exm.Error)
		} else {
			ex.meta.Error = exm.Error
		}
	} else if onSuccess != nil {
		onSuccess(ex.meta)
	}
	ex.onUpdateHook()

	return exm.Error
}
