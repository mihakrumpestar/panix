package executioner

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertFreshOverrideOrdering pins the load-bearing composition order shared
// by the composition and end-to-end tests: the override options come after
// the multiplexing defaults (ssh honors the last value of a repeated option)
// and before the target, so the argv stays valid.
func assertFreshOverrideOrdering(t *testing.T, got []string, target string) {
	t.Helper()

	autoMasterIdx := slices.Index(got, "ControlMaster=auto")
	require.NotEqual(t, -1, autoMasterIdx, "client args keep the multiplexing default")

	freshMasterIdx := slices.Index(got, "ControlMaster=no")
	require.NotEqual(t, -1, freshMasterIdx)

	autoPathIdx := slices.IndexFunc(got, func(arg string) bool {
		return strings.HasPrefix(arg, "ControlPath=") && arg != "ControlPath=none"
	})
	require.NotEqual(t, -1, autoPathIdx)

	nonePathIdx := slices.Index(got, "ControlPath=none")
	require.NotEqual(t, -1, nonePathIdx)

	targetIdx := slices.Index(got, target)
	require.NotEqual(t, -1, targetIdx)

	assert.Greater(t, freshMasterIdx, autoMasterIdx, "the override must come after ControlMaster=auto")
	assert.Greater(t, nonePathIdx, autoPathIdx, "the override must come after the multiplexed ControlPath")
	assert.Less(t, freshMasterIdx, targetIdx, "options must precede the target")
}

// TestSSHCommandWithArgs_FreshConnectionOverridesMaster pins the fresh
// connection composition for both client shapes: the disabling options ride
// after MaybeSSHCommandArguments, so the later -o flags override
// ControlMaster=auto and the ControlPath socket.
func TestSSHCommandWithArgs_FreshConnectionOverridesMaster(t *testing.T) {
	t.Parallel()

	t.Run("non-alias client", func(t *testing.T) {
		t.Parallel()

		client := ssh.SSHClient{Hostname: "10.0.0.1", Port: 22, Username: "deploy"}

		got := sshCommandWithArgsOptions(
			client,
			[]string{"panix-guard", "ctl", "confirm"},
			sshArgvOptions{freshConnection: true},
		)

		assertFreshOverrideOrdering(t, got, "10.0.0.1")
		assert.Equal(t, []string{`'panix-guard'`, `'ctl'`, `'confirm'`}, got[len(got)-3:])
	})

	t.Run("alias client", func(t *testing.T) {
		t.Parallel()

		client := &ssh.SSHClient{}
		require.NoError(t, client.Init("panix-alias-test-machine", "panix-other-local", nixver.Info{}))

		got := sshCommandWithArgsOptions(*client, []string{"echo", "hi"}, sshArgvOptions{freshConnection: true})

		assertFreshOverrideOrdering(t, got, "panix-alias-test-machine")
	})
}

// TestSSHCommandWithArgs_ZeroOptionsKeepMaster pins the zero-value contract:
// without FreshConnection the argv carries no override options at all.
func TestSSHCommandWithArgs_ZeroOptionsKeepMaster(t *testing.T) {
	t.Parallel()

	client := ssh.SSHClient{Hostname: "10.0.0.1", Port: 22, Username: "deploy"}

	got := sshCommandWithArgs(client, []string{"echo", "hi"})

	assert.NotContains(t, got, "ControlMaster=no")
	assert.NotContains(t, got, "ControlPath=none")
	assert.Contains(t, got, "ControlMaster=auto")
}

// TestExec_RemoteFreshConnectionArgv pins the end-to-end remote route: the
// FreshConnection option reaches the spawned ssh argv appended after the
// multiplexing defaults; without the option the argv keeps today's shape.
//
//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestExec_RemoteFreshConnectionArgv(t *testing.T) {
	tests := []struct {
		name      string
		opts      []ExecOption
		wantFresh bool
	}{
		{name: "fresh connection override", opts: []ExecOption{FreshConnection()}, wantFresh: true},
		{name: "default keeps the master", opts: nil, wantFresh: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argvPath, _ := stubSSHEnvDir(t, `#!/bin/sh
{
  printf '== invocation\n'
  printf '%s\n' "$0" "$@"
} >> "$PIPE_SSH_ARGV_FILE"
`)

			exc, _ := newLocalExecutioner(t, newRemoteExecMachine(t))

			require.NoError(t, exc.Exec(
				"ctl confirm", "confirming", "confirm failed",
				[]string{"panix-guard", "ctl", "confirm"},
				tt.opts...,
			))

			invocations := sshStubInvocations(t, string(readTestFile(t, argvPath)))
			require.Len(t, invocations, 1)

			if tt.wantFresh {
				assertFreshOverrideOrdering(t, invocations[0], "10.0.0.1")
				assert.Equal(t, []string{`'panix-guard'`, `'ctl'`, `'confirm'`}, invocations[0][len(invocations[0])-3:])
			} else {
				assert.NotContains(t, invocations[0], "ControlMaster=no")
				assert.NotContains(t, invocations[0], "ControlPath=none")
			}
		})
	}
}

// assertKilledFast pins that the command was terminated by the effective
// timeout instead of finishing on its own; the exact error text depends on
// whether the PTY read or the wait observes the kill first, so only the
// fact of termination is asserted.
func assertKilledFast(t *testing.T, err error, start time.Time) {
	t.Helper()

	require.Error(t, err, "the command must be terminated by the timeout")
	assert.Less(t, time.Since(start), 10*time.Second, "the timeout must fire early")
}

// TestExec_WithTimeout pins the per-command timeout override: a short
// WithTimeout kills the command even under a long configured timeout, and
// the zero value keeps the configured timeout in charge.
func TestExec_WithTimeout(t *testing.T) {
	t.Parallel()

	t.Run("short override kills under a long configured timeout", func(t *testing.T) {
		t.Parallel()

		exc, _ := newLocalExecutioner(t, newLocalExecMachine(t), func(conf *ExecutionerConf) {
			conf.Timeout = time.Minute
		})

		start := time.Now()
		assertKilledFast(t, exc.Exec("sleep", "sleeping", "sleep failed", []string{"sleep", "30"}, WithTimeout(50*time.Millisecond)), start)
	})

	t.Run("zero value keeps the configured timeout", func(t *testing.T) {
		t.Parallel()

		exc, _ := newLocalExecutioner(t, newLocalExecMachine(t), func(conf *ExecutionerConf) {
			conf.Timeout = 50 * time.Millisecond
		})

		start := time.Now()
		assertKilledFast(t, exc.Exec("sleep", "sleeping", "sleep failed", []string{"sleep", "30"}), start)

		// The override wins over the configured timeout: the same command
		// length completes once the override raises the bound.
		require.NoError(t, exc.Exec("sleep", "sleeping", "sleep failed", []string{"sleep", "0.2"}, WithTimeout(time.Minute)))
	})
}

// TestExecPipe_WithTimeoutFires pins that the override reaches the pipe: a
// short override terminates the hanging source even under a long configured
// timeout, so the probe and write never run.
func TestExecPipe_WithTimeoutFires(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t), func(conf *ExecutionerConf) {
		conf.Timeout = time.Minute
	})

	start := time.Now()

	err := exc.ExecPipe(
		"hanging source", "streaming", "pipe failed",
		PipeSpec{
			Source: []string{"sleep", "30"},
			Probe:  []string{"sh", "-c", "printf probe"},
			Write:  []string{"sh", "-c", "cat"},
		},
		WithTimeout(50*time.Millisecond),
	)

	assertKilledFast(t, err, start)
}

// TestExec_Quiet pins the quiet contract: no phase-log entry for the exec,
// while output capture and the exit hooks stay fully functional, and the
// next ordinary exec registers normally.
func TestExec_Quiet(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	var captured string

	err := exc.Exec(
		"quiet echo", "echoing", "echo failed",
		[]string{"sh", "-c", "echo quiet-output"},
		Quiet(),
		OnSuccess(func(commandLog *command.CommandLog) error {
			captured = commandLog.Output.String()

			return nil
		}),
		OnDryRun(func() {}),
	)
	require.NoError(t, err)

	assert.Contains(t, captured, "quiet-output", "the hook must still receive the captured output")
	assert.Zero(t, phaseLog.CommandLogs.Length(), "a quiet exec must not register a phase-log command")

	require.NoError(t, exc.Exec("loud echo", "echoing", "echo failed", []string{"sh", "-c", "echo loud"}))

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok, "a non-quiet exec registers normally")
	assert.Contains(t, last.Output.String(), "loud")
}

// TestExec_QuietFailureStillReports pins that quiet does not swallow
// failures: OnFailure receives the consolidated error and the captured
// output, with still no phase-log entry.
func TestExec_QuietFailureStillReports(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	var (
		hookErr  error
		captured string
	)

	err := exc.Exec(
		"quiet fail", "failing", "fail failed",
		[]string{"sh", "-c", "echo before-crash; exit 3"},
		Quiet(),
		OnFailure(func(commandLog *command.CommandLog, execErr error) error {
			hookErr = execErr
			captured = commandLog.Output.String()

			return execErr
		}),
		OnDryRun(func() {}),
	)
	require.Error(t, err)

	assert.ErrorIs(t, err, hookErr, "the original error must reach the caller")
	assert.Contains(t, captured, "before-crash")
	assert.Zero(t, phaseLog.CommandLogs.Length(), "a quiet exec must not register a phase-log command")
}

// TestExecPipe_RejectsQuietAndFreshConnection pins that the Exec-only
// options fail loudly on the pipe path instead of being ignored.
func TestExecPipe_RejectsQuietAndFreshConnection(t *testing.T) {
	t.Parallel()

	spec := PipeSpec{
		Source: []string{"sh", "-c", "printf secret"},
		Probe:  []string{"sh", "-c", "true"},
		Write:  []string{"sh", "-c", "cat"},
	}

	tests := []struct {
		name    string
		opt     ExecOption
		wantErr string
	}{
		{name: "Quiet", opt: Quiet(), wantErr: "Quiet"},
		{name: "FreshConnection", opt: FreshConnection(), wantErr: "FreshConnection"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

			err := exc.ExecPipe("unsupported option", "running", "failed", spec, tt.opt)
			require.ErrorContains(t, err, tt.wantErr)
			assert.Zero(t, phaseLog.CommandLogs.Length(), "a rejected option must not create a CommandLog entry")
		})
	}
}
