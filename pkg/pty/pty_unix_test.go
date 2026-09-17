//go:build linux || darwin || freebsd

package pty

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readUntilEOF drains the PTY and returns all output. It fails the test if
// the stream ends with anything other than an unwrapped io.EOF.
func readUntilEOF(t *testing.T, ptyFile *Pty) string {
	t.Helper()

	var output bytes.Buffer

	buf := make([]byte, 1024)

	for {
		bytesRead, err := ptyFile.Read(buf)
		output.Write(buf[:bytesRead])

		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			require.NotContains(t, err.Error(), "pty: read")

			return output.String()
		}

		require.NotZero(t, bytesRead, "end-of-stream must surface as io.EOF, never a no-progress read")
	}
}

// TestPtyRead_BareEOFIsNormalized pins that a bare io.EOF from the master read
// (how darwin/FreeBSD surface a closed slave) is translated to an unwrapped
// io.EOF, covering the macOS path on Linux CI.
func TestPtyRead_BareEOFIsNormalized(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	require.NoError(t, err)

	defer func() { require.NoError(t, devNull.Close()) }()

	p := &Pty{master: devNull}

	buf := make([]byte, 1024)

	bytesRead, err := p.Read(buf)

	assert.Zero(t, bytesRead)
	require.ErrorIs(t, err, io.EOF)
	require.NotContains(t, err.Error(), "pty: read")
}

// TestPtyRead_EndOfStream pins that a finished child surfaces as an unwrapped
// io.EOF read on every platform and that the exit status comes from Wait.
func TestPtyRead_EndOfStream(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		command      string
		wantOutput   string
		wantExitCode int
	}{
		{"exit zero", "printf 'hello'", "hello", 0},
		{"non-zero exit", "exit 7", "", 7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// #nosec G204 -- command is a fixed, test-owned table case
			cmd := exec.CommandContext(t.Context(), "sh", "-c", test.command)

			p, err := Start(cmd)
			require.NoError(t, err)

			defer func() { require.NoError(t, p.Close()) }()

			output := readUntilEOF(t, p)

			if test.wantOutput != "" {
				assert.Contains(t, output, test.wantOutput)
			}

			if test.wantExitCode == 0 {
				require.NoError(t, cmd.Wait())

				return
			}

			var exitErr *exec.ExitError

			require.ErrorAs(t, cmd.Wait(), &exitErr)
			assert.Equal(t, test.wantExitCode, exitErr.ExitCode())
		})
	}
}
