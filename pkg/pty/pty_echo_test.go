//go:build linux || darwin || freebsd

package pty

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runEchoSession runs one echo phase in a fresh PTY: prepare adjusts the
// echo state before any input exists, so the flag is fixed for every byte
// the line discipline ever receives in this session. The child consumes one
// line and exits, which ends the master stream in io.EOF.
func runEchoSession(t *testing.T, prepare func(*Pty) error, line string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "sh", "-c", "read line")

	p, err := Start(cmd)
	require.NoError(t, err)

	defer func() { require.NoError(t, p.Close()) }()

	if prepare != nil {
		require.NoError(t, prepare(p))
	}

	_, err = p.Write([]byte(line + "\n"))
	require.NoError(t, err)

	output := readUntilEOF(t, p)
	require.NoError(t, cmd.Wait())

	return output
}

// TestPtySetEcho_EchoStateBeforeFirstInput pins the echo control behaviorally
// with the flag fixed before the line discipline ever sees input, so no
// asynchronous echo processing can straddle a flag change. The empty output
// in the disabled phase also pins that ECHONL is cleared together with ECHO:
// a leftover ECHONL would surface the frame's line feed as a bare echoed
// line break.
func TestPtySetEcho_EchoStateBeforeFirstInput(t *testing.T) {
	t.Parallel()

	t.Run("default echoes input", func(t *testing.T) {
		t.Parallel()

		output := runEchoSession(t, nil, "echoed")

		assert.Contains(t, output, "echoed")
	})

	t.Run("SetEcho false silences echo and ECHONL", func(t *testing.T) {
		t.Parallel()

		output := runEchoSession(t, func(p *Pty) error { return p.SetEcho(false) }, "silent")

		assert.Empty(t, output, "no echo and no ECHONL line feed may reach the master")
	})

	t.Run("SetEcho true after false restores echo", func(t *testing.T) {
		t.Parallel()

		output := runEchoSession(t, func(p *Pty) error {
			err := p.SetEcho(false)
			if err != nil {
				return err
			}

			return p.SetEcho(true)
		}, "restored")

		assert.Contains(t, output, "restored")
	})
}

// TestPtySetEcho_MidStreamToggle pins the production pattern: SetEcho(false)
// after Start, while the session is live. The line discipline decides about
// echo asynchronously, so the phases are synchronized through the child: a
// frame counts as processed only once the child has consumed it and
// acknowledged it on the same FIFO output stream. The next frame is written
// strictly after the flag change, so its receive must use the new state.
func TestPtySetEcho_MidStreamToggle(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "sh", "-c", `read a; echo ACK1; read b; echo ACK2; read c; echo ACK3`)

	p, err := Start(cmd)
	require.NoError(t, err)

	defer func() { require.NoError(t, p.Close()) }()

	chunks := make(chan string, 64)
	readerDone := make(chan struct{})

	go func() {
		defer close(readerDone)

		buf := make([]byte, 1024)

		for {
			bytesRead, readErr := p.Read(buf)
			if bytesRead > 0 {
				chunks <- string(buf[:bytesRead])
			}

			if readErr != nil {
				return
			}
		}
	}()

	var accumulated strings.Builder

	waitFor := func(marker string) {
		t.Helper()

		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()

		for !strings.Contains(accumulated.String(), marker) {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					require.FailNowf(t, "stream ended early", "no %q in %q", marker, accumulated.String())
				}

				accumulated.WriteString(chunk)
			case <-timeout.C:
				require.FailNowf(t, "timeout", "no %q in %q", marker, accumulated.String())
			}
		}
	}

	_, err = p.Write([]byte("echo-on\n"))
	require.NoError(t, err)
	waitFor("ACK1")

	require.NoError(t, p.SetEcho(false))

	_, err = p.Write([]byte("echo-off\n"))
	require.NoError(t, err)
	waitFor("ACK2")

	require.NoError(t, p.SetEcho(true))

	_, err = p.Write([]byte("echo-again\n"))
	require.NoError(t, err)
	waitFor("ACK3")

	select {
	case <-readerDone:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the child must exit after consuming all three lines")
	}

	// Drain the reader's leftover chunks before counting. waitFor stops
	// consuming at the marker, and the reader keeps reading until EOF, so
	// trailing chunks (including final line breaks) can still sit in the
	// buffered channel under scheduling pressure. readerDone guarantees the
	// reader has returned, hence no further sends are possible, and EOF
	// guarantees the kernel delivers no more master data: a non-blocking
	// sweep reaches output quiescence deterministically. Without it the
	// count below can miss breaks and flake under package-level load.
drain:
	for {
		select {
		case chunk := <-chunks:
			accumulated.WriteString(chunk)
		default:
			break drain
		}
	}

	output := accumulated.String()

	assert.Contains(t, output, "echo-on")
	assert.NotContains(t, output, "echo-off", "a frame written with echo disabled must not be echoed")
	assert.Contains(t, output, "echo-again")
	assert.Equal(t, 5, strings.Count(output, "\r\n"),
		"exactly the two echoed frames and the three acknowledgments produce line breaks; a leftover ECHONL would add one")

	require.NoError(t, cmd.Wait())
}
