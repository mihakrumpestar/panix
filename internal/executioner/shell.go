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

func (ex *Executioner) shellStream(onFailure func(*config.Log, error) error, onSuccess func(*config.Log) error, name string, args ...string) error {
	if ex.log.Commands == nil {
		ex.log.Commands = make([]*config.CommandLog, 0)
	}

	exm := &config.CommandLog{TimeAndState: &config.TimeAndState{}}
	ex.log.Commands = append(ex.log.Commands, exm)

	// prepare initial event
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

	exm.StartTimer()
	ex.onUpdateHook()

	// Start the process with PTY
	ptyMaster, err := pty.Start(cmd)
	if err != nil {
		exm.EndTimerWithError(err)
		ex.onUpdateHook()
		return err
	}

	defer func() {
		errPty := ptyMaster.Close()
		if errPty != nil && err != nil {
			errPty = errors.Wrap(errPty, err.Error())
		} else if err != nil {
			errPty = err
		}
		exm.EndTimerWithError(errPty)
		ex.onUpdateHook()
	}()

	// Read from PTY and capture to buffer in real-time
	buf := make([]byte, 8192)
	var readErr error

	for {
		var n int
		n, readErr = ptyMaster.Read(buf)
		if readErr != nil {
			readErr = ptyError(readErr)
			if readErr != nil && readErr != io.EOF {
				exm.StdInOutErr.WriteString("PTY read error: " + readErr.Error() + "\n")
			}

			break
		}

		if n > 0 {
			// Process the buffer to handle every \r character (some msgs have more than one)
			processed := bytes.ReplaceAll(buf[:n], []byte("\r"), []byte(""))

			// Check if there's actual content
			cleanedAndTrimmed := CleanAnsiAndSpace(processed)

			// Only add to log if there is actual content in buffer
			if len(cleanedAndTrimmed) == 0 {
				continue
			}

			// Only add newline if it's not an ANSI escape sequence
			if !bytes.HasPrefix(processed, []byte{27}) { // 27 is ESC character
				processed = append(processed, []byte("\n")...)
			}

			// Debug ANSI
			//processed = []byte(strconv.Quote(string(processed)))

			_, err = exm.StdInOutErr.Write(processed)
			if err != nil {
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
	exm.EndTimerWithError(err)
	ex.onUpdateHook()

	return err
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

func CleanAnsiAndSpace(b []byte) []byte {
	// ANSI escape sequence regex pattern - matches common escape sequences
	// Handles: \x1b[K (erase line), \x1b[...m (colors), \x1b[...A/B/C/D (cursor), etc.
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// Remove ANSI escape sequences
	cleaned := ansiRegex.ReplaceAll(b, []byte{})

	cleanedAndTrimmed := bytes.TrimSpace(cleaned)

	return cleanedAndTrimmed
}
