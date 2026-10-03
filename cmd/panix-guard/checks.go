package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// envWithPath replaces PATH in base with the conservative path (spec 6.4): the sshd
// non-interactive shell PATH and sudo secure_path cannot be relied on.
func envWithPath(base []string, path string) []string {
	out := make([]string, 0, len(base)+1)
	for _, e := range base {
		if !strings.HasPrefix(e, "PATH=") {
			out = append(out, e)
		}
	}

	return append(out, "PATH="+path)
}

// runLocalCheck executes one user health check (spec 6.2): sh -c with the conservative
// PATH, bounded by checkTimeout, output teed into the log (and the live wire).
// Failure excerpts name the check; the full output stays in the log.
func runLocalCheck(command string, write func(string)) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	// The check command is operator configuration (spec 2: health_checks_local).
	c := exec.CommandContext(ctx, "/bin/sh", "-c", command) //nolint:gosec // user-configured check command by design (spec 2)
	c.Env = envWithPath(os.Environ(), conservativePath)

	out, err := c.CombinedOutput()
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if line != "" {
			write("check: " + line)
		}
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return false, fmt.Sprintf("check timed out after %s: %s", checkTimeout, command)
	}

	if err != nil {
		return false, fmt.Sprintf("check failed (rc=%d): %s", exitCodeOf(err), command)
	}

	return true, ""
}

// systemctlCandidates lists the systemctl probe locations in preference order
// (spec 6.4): PATH first (so environments can provide their own), then the current
// system profile as the reliable fallback when the inherited PATH is unreliable.
func systemctlCandidates() []string {
	return []string{"systemctl", "/run/current-system/sw/bin/systemctl"}
}

// captureFailedUnits snapshots the failed systemd units (unit names only). ok is false
// when no candidate works; the builtin check disables itself in that case (spec 6.2).
func captureFailedUnits(logw *LogWriter) (map[string]bool, bool) {
	for _, cand := range systemctlCandidates() {
		if _, err := exec.LookPath(cand); err != nil {
			continue
		}

		c := exec.Command(cand, "list-units", "--state=failed", "--plain", "--no-legend") //nolint:gosec // the candidate list is a fixed probe table
		c.Env = envWithPath(os.Environ(), conservativePath)

		out, err := c.Output()
		if err != nil {
			continue
		}

		units := make(map[string]bool)

		for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			fields := strings.Fields(line)
			units[fields[0]] = true
		}

		return units, true
	}

	return nil, false
}
