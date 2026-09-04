package inspect

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Exceeds less's 24-line fallback screen, so paging would trigger.
	ptyRegressionGenerations = 25
	ptyRegressionDeadline    = 30 * time.Second
)

// TestReadGenerations_PTYPagerRegression guards issue #14: nix-env pages
// on TTY stdout and panix's PTY never feeds the pager, so 23+ generations
// hang local inspect forever. argv mirrors readGenerations; keep in sync.
func TestReadGenerations_PTYPagerRegression(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}

	_, err := exec.LookPath("nix-env")
	if err != nil {
		t.Skip("nix-env not available")
	}

	profile := fakeGenerationsProfile(t, ptyRegressionGenerations)

	argv := []string{"env", "NIX_PAGER=cat", "nix-env", "--profile", profile, "--list-generations"}
	// #nosec G204 -- argv is built here from a test-owned profile path
	cmd := exec.CommandContext(t.Context(), argv[0], argv[1:]...)
	// Prefer a pager with a valid TERM: without NIX_PAGER=cat, less runs interactively.
	cmd.Env = append(filterEnv(os.Environ(), "TERM", "PAGER"), "TERM=xterm-256color", "PAGER=less")

	ptyFile, err := pty.Start(cmd)
	require.NoError(t, err)

	defer ptyFile.Close() //nolint:errcheck // best-effort cleanup

	outputCh := make(chan string, 1)
	go drainPty(ptyFile, outputCh)

	var output string

	select {
	case output = <-outputCh:
	case <-time.After(ptyRegressionDeadline):
		_ = ptyFile.Close()
		_, _ = cmd.Process.Wait()

		t.Fatal("nix-env hung on the PTY: nix's pager waits for input panix never provides (issue #14)")
	}

	require.NoError(t, cmd.Wait())
	require.NotEmpty(t, output)
	assert.Contains(t, output, "25", "all generations must be listed")
	assert.NotContains(t, output, "lines 1-", "less status line must never appear")
}

// fakeGenerationsProfile creates a throwaway nix profile with count generations.
func fakeGenerationsProfile(t *testing.T, count int) string {
	t.Helper()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(target, 0o750))

	for gen := 1; gen <= count; gen++ {
		require.NoError(t, os.Symlink(target, filepath.Join(dir, "profile-"+strconv.Itoa(gen)+"-link")))
	}

	profile := filepath.Join(dir, "profile")
	require.NoError(t, os.Symlink(target, profile))

	return profile
}

// drainPty mirrors the executioner: blocking reads, no input ever written.
func drainPty(ptyFile *pty.Pty, outputCh chan<- string) {
	var captured strings.Builder

	buf := make([]byte, 8192)

	for {
		n, readErr := ptyFile.Read(buf)
		if n > 0 {
			captured.Write(buf[:n])
		}

		if readErr != nil || n == 0 {
			outputCh <- captured.String()

			return
		}
	}
}

// filterEnv removes vars from env so test overrides are not shadowed
// (getenv honors the first match).
func filterEnv(env []string, vars ...string) []string {
	return slices.DeleteFunc(env, func(entry string) bool {
		return slices.ContainsFunc(vars, func(v string) bool {
			return strings.HasPrefix(entry, v+"=")
		})
	})
}
