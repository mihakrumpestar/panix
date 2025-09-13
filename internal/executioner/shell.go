package executioner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
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
	buf := make([]byte, 10*8192)
	var readErr error

	// Warning: the folowing read sequance applies to pty.stdin too
	for {
		var n int
		n, readErr = exm.Pty.Read(buf)
		if readErr != nil {
			readErr = ptyError(readErr)
			if readErr != nil && readErr != io.EOF {
				exm.WriteString("PTY read error: " + readErr.Error() + "\n")
			}

			break
		}

		if n > 0 {
			rawBuffer := buf[:n]

			// Process the buffer to handle terminal control sequences properly
			// This makes it work like a normal terminal would
			processTerminalOutput(rawBuffer, exm)

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

// processTerminalOutputLine processes a single line of terminal output to handle control sequences
// like a real terminal would, but in a way that's safe for TUI display
func processTerminalOutput(buf []byte, exm *config.CommandLog) {
	// Convert the buffer to a string for easier processing
	str := string(buf)

	// Split by newlines first
	lines := strings.Split(str, "\n")

	// Buffer was cut off, stiching together
	length := len(lines)
	if len(lines[length-1]) == 0 {
		lines = lines[:length-1]
	}

	for index, line := range lines {

		if len(CleanAnsiAndSpace([]byte(line))) == 0 {
			continue
		}

		writeIndex := func() {
			line = strings.ReplaceAll(line, "\x1b[K", "")

			if index == 0 {
				exm.WriteString(line)
			} else {
				exm.WriteLine([]byte(line))
			}
		}

		// Normal line
		if strings.HasSuffix(line, "\r") {
			line = strings.TrimSuffix(line, "\r")

			line = strings.ReplaceAll(line, "\x1b[K", "")

			writeIndex()
			continue
		}

		if strings.HasPrefix(line, "\r") {
			line = strings.TrimPrefix(line, "\r")

			line = strings.ReplaceAll(line, "\x1b[K", "")

			exm.WriteLine([]byte("r present: " + strconv.Quote(line)))
			exm.WriteLine([]byte(""))

			// In case multiple \r in sinble line, take the last sequence
			if strings.Contains(line, "\r") {
				rSplitted := strings.Split(line, "\r")

				line = rSplitted[len(rSplitted)-1]
			}

			//exm.WriteLine([]byte("R present: " + strconv.Quote(line)))
			//exm.WriteLine([]byte(""))

			if strings.Contains(line, "\x1b[K") { // clear line
				line = strings.ReplaceAll(line, "\x1b[K", "")
				exm.ReplaceLastLine([]byte(line))
			} else {
				writeIndex()
			}

			continue
		}

		if index == 0 {
			writeIndex()
			continue
		}

		if index == len(lines)-1 {
			writeIndex()
			continue
		}

		exm.WriteLine([]byte("UNPROCESSED (" + fmt.Sprintf("%d/%d", index, len(lines)) + "): " + strconv.Quote(line)))
	}
}

func CleanAnsiAndSpace(b []byte) []byte {
	// ANSI escape sequence regex pattern - matches common escape sequences
	// Handles: \x1b[K (erase line), \x1b[...m (colors), \x1b[...A/B/C/D (cursor), etc.
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// Remove ANSI escape sequences
	cleaned := ansiRegex.ReplaceAll(b, []byte{})

	cleanedAndTrimmed := bytes.TrimSpace(cleaned)

	return cleanedAndTrimmed
}
