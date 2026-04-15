package executioner

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"syscall"

	"github.com/creack/pty"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/pkg/errors"
)

var ErrWaitAndReadError = errors.New("wait and read error")

const (
	ptyBufferSize = 8192
)

// ANSI escape sequence regex pattern - matches common escape sequences.
// Handles: \x1b[K (erase line), \x1b[...m (colors), \x1b[...A/B/C/D (cursor), etc.
// Also handles OSC (Operating System Command) sequences like \x1b]0;...BEL.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][0-9;]*;[^\x07\x1b]*[\x07\x1b\\]`)

func (ex *Executioner) shellStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	err := validateExecOptions(excOpt)
	if err != nil {
		return err
	}

	commandLog := ex.phaseLog.NewCommand(description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt.env)

	endLog := ex.startCommandLog(commandLog, description, statusIfRunning, commandLog.Command)

	var execErr error

	defer func() {
		endLog(execErr, commandLog)
	}()

	cmdCtx, cancel := context.WithTimeout(ex.ctx, ex.timeout)
	defer cancel()

	cmd := ex.prepareCommandWithEnv(cmdCtx, commandWithArgs, excOpt)
	ex.onUpdateHook()

	if ex.dryRun {
		return ex.handleDryRun(excOpt)
	}

	ptyFile, err := pty.Start(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to start pty")
	}

	defer func() { _ = ptyFile.Close() }()

	readErr := ex.readPTYOutput(cmdCtx, ptyFile, commandLog)

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

func (ex *Executioner) readPTYOutput(ctx context.Context, ptyFile *os.File, commandLog *command.CommandLog) error {
	buf := make([]byte, ptyBufferSize)

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context canceled")
		default:
			bytesRead, err := ptyFile.Read(buf)
			if err != nil {
				err = ex.handleReadError(err, commandLog)
				commandLog.TrimTrailingEmptyLines()

				return err
			}

			if bytesRead == 0 {
				continue
			}

			err = processTerminalOutput(buf[:bytesRead], commandLog)
			if err != nil {
				commandLog.WriteLineString("processTerminalOutput write error: " + err.Error())
				commandLog.WriteLineString("")

				return err
			}

			ex.onUpdateHook()
		}
	}
}

func (ex *Executioner) handleReadError(err error, commandLog *command.CommandLog) error {
	err = ptyError(err)
	if err != nil && !errors.Is(err, io.EOF) {
		commandLog.WriteLineString("PTY read error: " + err.Error())
		commandLog.WriteLineString("")
	}

	return err
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

// Helpers

// https://github.com/owenthereal/upterm/pull/11/files
// Linux kernel return EIO when attempting to read from a master pseudo
// terminal which no longer has an open slave. So ignore error here.
// See https://github.com/creack/pty/issues/21.
func ptyError(err error) error {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr == nil {
		return err
	}

	if !errors.Is(pathErr.Err, syscall.EIO) {
		return err
	}

	return nil
}

// processTerminalOutput processes terminal output to handle control sequences
// like a real terminal would, but in a way that's safe for TUI display.
func processTerminalOutput(buf []byte, exm *command.CommandLog) error {
	buf = bytes.ReplaceAll(buf, []byte("\x1b[K"), nil)
	sequences := bytes.Split(buf, []byte("\r"))

	for i, seq := range sequences {
		err := processSequence(seq, i == 0, exm)
		if err != nil {
			return err
		}
	}

	return nil
}

func processSequence(seq []byte, isFirst bool, exm *command.CommandLog) error {
	if after, ok := bytes.CutPrefix(seq, []byte("\n")); ok {
		exm.WriteLine(after)

		return nil
	}

	if isFirst {
		_, err := exm.Write(seq)
		if err != nil {
			return errors.Wrap(err, "failed to write to command log")
		}

		return nil
	}

	exm.ReplaceLastLine(seq)

	return nil
}

func CleanAnsiAndSpace(b []byte) []byte {
	return bytes.TrimSpace(ansiRegex.ReplaceAll(b, nil))
}
