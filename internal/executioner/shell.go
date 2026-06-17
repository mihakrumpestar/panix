package executioner

import (
	"context"
	"os"
	"os/exec"

	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/pty"
	"github.com/pkg/errors"
)

var ErrWaitAndReadError = errors.New("wait and read error")

const ptyBufferSize = 8192

func (ex *Executioner) shellStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	err := validateExecOptions(excOpt)
	if err != nil {
		return err
	}

	commandLog := ex.conf.PhaseLog.NewCommand(ex.phaseXpath, description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt.env)
	endLog := ex.startCommandLog(commandLog, description, statusIfRunning, commandLog.Command)

	var execErr error

	defer func() {
		endLog(execErr, commandLog)
	}()

	cmdCtx, cancel := context.WithTimeout(ex.conf.Ctx, ex.conf.Timeout)
	defer cancel()

	cmd := ex.prepareCommandWithEnv(cmdCtx, commandWithArgs, excOpt)
	ex.conf.OnUpdateHook()

	if ex.conf.DryRun {
		return ex.handleDryRun(excOpt)
	}

	ptyFile, err := pty.Start(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to start pty")
	}

	defer func() { _ = ptyFile.Close() }()

	readErr := ex.readPTYOutput(cmdCtx, ptyFile, commandLog)
	finalizeCommandLog(commandLog)

	execErr = ex.finalizeExecution(cmd, readErr, commandLog, excOpt)
	if execErr != nil {
		return errors.Wrap(execErr, statusIfFailed)
	}

	return nil
}

func (ex *Executioner) prepareCommandWithEnv(ctx context.Context, commandWithArgs []string, excOpt *ExecOptions) *exec.Cmd {
	// #nosec G204 -- commandWithArgs comes from internal configuration, not user input
	cmd := exec.CommandContext(ctx, commandWithArgs[0], commandWithArgs[1:]...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, excOpt.env...)

	return cmd
}

func (ex *Executioner) handleDryRun(excOpt *ExecOptions) error {
	if excOpt.onDryRun == nil {
		if excOpt.onSuccess != nil {
			return errors.New("OnDryRun is mandatory when OnSuccess is provided - please provide dry-run handling")
		}

		return nil
	}

	excOpt.onDryRun()

	return nil
}

func (ex *Executioner) readPTYOutput(ctx context.Context, ptyFile *pty.Pty, commandLog *command.CommandLog) error {
	buf := make([]byte, ptyBufferSize)
	proc := terminalProcessor{output: commandLog.Output}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context canceled")
		default:
			bytesRead, err := ptyFile.Read(buf)
			if err != nil {
				commandLog.Output.Write([]byte("PTY read error: " + err.Error()))
				commandLog.Output.Write([]byte{})

				return errors.Wrap(err, "pty read")
			}

			if bytesRead == 0 {
				return nil
			}

			proc.process(buf[:bytesRead], commandLog)

			ex.conf.OnUpdateHook()
		}
	}
}

func (ex *Executioner) finalizeExecution(
	cmd *exec.Cmd,
	readErr error,
	commandLog *command.CommandLog,
	excOpt *ExecOptions,
) error {
	waitErr := cmd.Wait()
	err := consolidateErrors(waitErr, readErr)

	if err != nil && excOpt.onFailure != nil {
		return excOpt.onFailure(commandLog, err)
	}

	if err == nil && excOpt.onSuccess != nil {
		return excOpt.onSuccess(commandLog)
	}

	return err
}

func validateExecOptions(excOpt *ExecOptions) error {
	if excOpt.onSuccess != nil && excOpt.onDryRun == nil {
		return errors.New("OnDryRun is mandatory when OnSuccess is provided - every command with OnSuccess must handle dry-run mode")
	}

	return nil
}

func consolidateErrors(waitErr, readErr error) error {
	switch {
	case waitErr != nil && readErr != nil:
		return errors.Wrapf(ErrWaitAndReadError, "wait=%v, read=%v", waitErr, readErr)
	case readErr != nil:
		return readErr
	default:
		return waitErr
	}
}
