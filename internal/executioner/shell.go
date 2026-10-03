package executioner

import (
	"bytes"
	"context"
	"io"
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

	// Trimming is opt-in: only commands that explicitly request it
	// (e.g. nix copy, nix build) get their output trimmed.
	var maxOutputLines uint64
	if excOpt.trim {
		maxOutputLines = ex.conf.MaxOutputLines
	}

	commandLog := ex.newCommandLog(
		excOpt, description, statusIfRunning, statusIfFailed,
		commandWithArgs, maxOutputLines,
	)
	endLog := ex.startCommandLog(commandLog, description, statusIfRunning, commandLog.Command)

	var execErr error

	defer func() {
		endLog(execErr, commandLog)
	}()

	cmdCtx, cancel := context.WithTimeout(ex.conf.Ctx, ex.execTimeout(excOpt))
	defer cancel()

	cmd := ex.prepareCommand(cmdCtx, commandWithArgs)
	ex.conf.OnUpdateHook()

	if ex.conf.DryRun {
		return ex.handleDryRun(excOpt)
	}

	ptyFile, err := pty.Start(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to start pty")
	}

	defer func() { _ = ptyFile.Close() }()

	// WithStdin feeds the PTY master from the caller's reader: the duplex
	// control channel of the activation guard. Echo is disabled BEFORE the
	// pump starts, so command frames are never echoed back into the inbound
	// stream: the echo decision is asynchronous in the line discipline, so
	// the flag must be fixed before the first frame exists. Best-effort: the
	// protocol's echo defense ignores command-shaped lines even when the
	// ioctl fails. Without WithStdin the line discipline stays untouched.
	if excOpt.stdin != nil {
		_ = ptyFile.SetEcho(false)

		_ = startStdinPump(cmdCtx, ptyFile, excOpt.stdin)
	}

	readErr := ex.readPTYOutputTap(cmdCtx, ptyFile, commandLog, excOpt.outputTap)
	finalizeCommandLog(commandLog)

	execErr = ex.finalizeExecution(cmd, readErr, commandLog, excOpt)
	if execErr != nil {
		return errors.Wrap(execErr, statusIfFailed)
	}

	return nil
}

func (ex *Executioner) prepareCommand(ctx context.Context, commandWithArgs []string) *exec.Cmd {
	// #nosec G204 -- commandWithArgs comes from internal configuration, not user input
	cmd := exec.CommandContext(ctx, commandWithArgs[0], commandWithArgs[1:]...)
	cmd.Env = os.Environ()

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

// readPTYOutput reads the PTY master until end-of-stream and processes the
// output into the command log with no output tap.
func (ex *Executioner) readPTYOutput(ctx context.Context, reader io.Reader, commandLog *command.CommandLog) error {
	return ex.readPTYOutputTap(ctx, reader, commandLog, nil)
}

// readPTYOutputTap is readPTYOutput with an optional raw output tap: every
// chunk read is handed to outputTap in stream order before terminal
// processing, and the tap never alters the command log path. A nil tap is
// the plain readPTYOutput behavior.
func (ex *Executioner) readPTYOutputTap(
	ctx context.Context,
	reader io.Reader,
	commandLog *command.CommandLog,
	outputTap func([]byte),
) error {
	buf := make([]byte, ptyBufferSize)
	proc := terminalProcessor{output: commandLog.Output}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context canceled")
		default:
			bytesRead, err := reader.Read(buf)

			// Process data before the error: a reader may report n > 0 together with an error.
			if bytesRead > 0 {
				// The tap sees the raw chunk before terminal processing, in
				// stream order, on this read goroutine. The chunk is copied
				// because the callback may retain it while buf is reused.
				if outputTap != nil {
					outputTap(bytes.Clone(buf[:bytesRead]))
				}

				proc.process(buf[:bytesRead], commandLog)

				ex.conf.OnUpdateHook()
			}

			if err != nil {
				if errors.Is(err, io.EOF) {
					// End-of-stream: exit status comes from cmd.Wait in finalizeExecution.
					return nil
				}

				commandLog.Output.Write([]byte("PTY read error: " + err.Error()))
				commandLog.Output.Write([]byte{})

				return errors.Wrap(err, "pty read")
			}

			if bytesRead == 0 {
				// PTY masters signal end-of-stream as io.EOF; this only guards a no-progress reader.
				return nil
			}
		}
	}
}

func (ex *Executioner) finalizeExecution(
	cmd *exec.Cmd,
	readErr error,
	commandLog *command.CommandLog,
	excOpt *ExecOptions,
) error {
	return finalizeExecError(consolidateErrors(cmd.Wait(), readErr), commandLog, excOpt)
}

// finalizeExecError applies the caller's OnFailure/OnSuccess hooks to the
// consolidated process error. It is shared by the PTY and pipe paths so both
// keep identical status handling.
func finalizeExecError(err error, commandLog *command.CommandLog, excOpt *ExecOptions) error {
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
