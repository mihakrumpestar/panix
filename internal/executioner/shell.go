package executioner

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/pkg/errors"
)

func (ex *Executioner) shellStream(commandWithArgs []string, excOpt *ExecOptions) (err error) {
	commandLog := ex.phaseLog.NewCommand()

	commandLog.TimeAndState.StartTimer()
	defer func() {
		commandLog.TimeAndState.EndTimerWithError(err)
		ex.onUpdateHook()
	}()

	command := commandWithArgs[0]
	args := commandWithArgs[1:]

	// Prepare initial event
	cmd := exec.CommandContext(ex.ctx, command, args...)
	cmd.Env = excOpt.env
	commandLog.Command.Store(strings.Join(cmd.Args, " "))
	ex.onUpdateHook()

	// dry-run short-circuit
	if ex.dryRun {
		if excOpt.onSuccess != nil {
			err := excOpt.onSuccess(commandLog)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Start the process with PTY
	commandLog.Pty, err = pty.Start(cmd)
	if err != nil {
		return
	}

	defer func() {
		errPty := commandLog.Pty.Close()
		if err != nil && errPty != nil {
			err = errors.Wrap(err, errPty.Error())
		} else if errPty != nil {
			err = errPty
		}
	}()

	// Read from PTY and capture to buffer in real-time
	buf := make([]byte, 8192)
	var readErr error

	// Warning: the folowing read sequance applies to pty.stdin too
	for {
		var n int
		n, readErr = commandLog.Pty.Read(buf)
		if readErr != nil {
			readErr = ptyError(readErr)
			if readErr != nil && readErr != io.EOF {
				commandLog.WriteLineString("PTY read error: " + readErr.Error())
				commandLog.WriteLineString("")
			}

			break
		}

		if n > 0 {
			rawBuffer := buf[:n]

			// Process the buffer to handle terminal control sequences properly
			// This makes it work like a normal terminal would
			readErr = processTerminalOutput(rawBuffer, commandLog)
			if readErr != nil {
				commandLog.WriteLineString("processTerminalOutput write error: " + readErr.Error())
				commandLog.WriteLineString("")

				break
			}

			ex.onUpdateHook()
		}
	}

	// Wait for command to complete
	err = cmd.Wait()
	if err != nil && readErr != nil {
		err = errors.Wrap(err, readErr.Error())
	} else if readErr != nil {
		err = readErr
	}
	if err != nil {
		if excOpt.onFailure != nil {
			err = excOpt.onFailure(commandLog, err)
		}
	} else if excOpt.onSuccess != nil {
		err = excOpt.onSuccess(commandLog)
	}

	return
}

// Helpers

// https://github.com/owenthereal/upterm/pull/11/files
// Linux kernel return EIO when attempting to read from a master pseudo
// terminal which no longer has an open slave. So ignore error here.
// See https://github.com/creack/pty/issues/21
func ptyError(err error) error {
	pathErr, ok := err.(*os.PathError)
	if !ok || pathErr.Err != syscall.EIO {
		return err
	}

	return nil
}

// processTerminalOutput processes terminal output to handle control sequences
// like a real terminal would, but in a way that's safe for TUI display
func processTerminalOutput(buf []byte, exm *logs.CommandLog) error {
	buf = bytes.ReplaceAll(buf, []byte("\x1b[K"), []byte{})

	// Split by position markers
	sequences := bytes.Split(buf, []byte("\r"))

	for index, sequence := range sequences {
		if bytes.HasPrefix(sequence, []byte("\n")) {
			sequence = bytes.TrimPrefix(sequence, []byte("\n"))
			exm.WriteLine(sequence)
		} else {
			if index == 0 { // Stitch together from previous buffer
				_, err := exm.Write(sequence)
				if err != nil {
					return err
				}

				continue
			}

			exm.ReplaceLastLine(sequence)
		}
	}

	return nil
}

func CleanAnsiAndSpace(b []byte) []byte {
	// ANSI escape sequence regex pattern - matches common escape sequences
	// Handles: \x1b[K (erase line), \x1b[...m (colors), \x1b[...A/B/C/D (cursor), etc.
	// Also handles OSC (Operating System Command) sequences like \x1b]0;...BEL
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][0-9;]*;[^\x07\x1b]*[\x07\x1b\\]`)
	cleaned := ansiRegex.ReplaceAll(b, []byte{})

	cleaned = bytes.TrimSpace(cleaned)

	return cleaned
}
