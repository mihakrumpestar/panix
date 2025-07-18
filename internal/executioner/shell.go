package executioner

import (
	"bufio"
	"os/exec"
	"strings"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
)

func (ex *Executioner) shellStream(onFailure func(*config.Log, error) error, onSuccess func(*config.Log), name string, args ...string) error {
	if ex.log.Commands == nil {
		ex.log.Commands = make([]*config.CommandLog, 0)
	}

	exm := &config.CommandLog{}
	ex.log.Commands = append(ex.log.Commands, exm)

	// prepare initial event
	cmd := exec.CommandContext(ex.ctx, name, args...)
	exm.Command = strings.Join(cmd.Args, " ")
	ex.onUpdateHook()

	// dry-run short-circuit
	if ex.dryRun {
		if onSuccess != nil {
			onSuccess(ex.log)
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
	exm.StartTimer()
	ex.onUpdateHook()

	err := execStream()

	if err != nil {
		if onFailure != nil {
			err = onFailure(ex.log, err)
		}
	} else if onSuccess != nil {
		onSuccess(ex.log)
	}
	exm.EndTimerWithError(err)
	ex.onUpdateHook()

	return err
}
