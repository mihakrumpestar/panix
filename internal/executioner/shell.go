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
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/pkg/errors"
)

func (ex *Executioner) shellStream(onFailure func(*config.Log, error) error, onSuccess func(*config.Log) error, name string, args ...string) (err error) {
	exm := ex.log.NewCommand()

	exm.TimeAndState.StartTimer()
	defer func() {
		exm.TimeAndState.EndTimerWithError(err)
		ex.onUpdateHook()
	}()

	// Prepare initial event
	cmd := exec.CommandContext(ex.ctx, name, args...)
	exm.Command = strings.Join(cmd.Args, " ")
	ex.onUpdateHook()

	// dry-run short-circuit
	if ex.dryRun {
		if onSuccess != nil {
			err := onSuccess(ex.log)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Start the process with PTY
	exm.Pty, err = pty.Start(cmd)
	if err != nil {
		return
	}

	defer func() {
		errPty := exm.Pty.Close()
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
		n, readErr = exm.Pty.Read(buf)
		if readErr != nil {
			readErr = ptyError(readErr)
			if readErr != nil && readErr != io.EOF {
				exm.WriteLineString("PTY read error: " + readErr.Error())
				exm.WriteLineString("")
			}

			break
		}

		if n > 0 {
			rawBuffer := buf[:n]

			// Process the buffer to handle terminal control sequences properly
			// This makes it work like a normal terminal would
			readErr = processTerminalOutput(rawBuffer, exm)
			if readErr != nil {
				exm.WriteLineString("processTerminalOutput write error: " + readErr.Error())
				exm.WriteLineString("")

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
		if onFailure != nil {
			err = onFailure(ex.log, err)
		}
	} else if onSuccess != nil {
		err = onSuccess(ex.log)
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
func processTerminalOutput(buf []byte, exm *config.CommandLog) error {
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
