package executioner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipeSecretMarker identifies the secret payload in assertions without
// requiring the whole binary payload to be printable.
const pipeSecretMarker = "PANIX-SECRET"

// secretPayload is deliberately hostile to line-oriented processing: NUL,
// ESC, tab, CR, high bytes and no trailing newline. Any transformation or
// buffering through a terminal processor would corrupt it.
func secretPayload() []byte {
	payload := []byte{0x00}
	payload = append(payload, pipeSecretMarker...)
	payload = append(payload,
		0x00,
		0x1b, '[', '3', '1', 'm', 'r', 'e', 'd', 0x1b, '[', '0', 'm',
		'\t', '\r', '\n',
		0x80, 0xff, 0xfe, 0x01, 0x7f,
	)

	return payload
}

// printfScript renders a POSIX sh script that writes payload to stdout. NUL
// bytes cannot travel through argv, so the payload is expressed as printf
// octal escapes inside the script text.
func printfScript(payload []byte) string {
	var script strings.Builder

	script.WriteString(`printf '%b' '`)

	for _, octet := range payload {
		fmt.Fprintf(&script, `\%03o`, octet)
	}

	script.WriteString(`'`)

	return script.String()
}

// writeToFileScript renders the argv that writes stdin into path.
func writeToFileScript(path string) []string {
	return []string{"sh", "-c", `cat > "$1"`, "--", path}
}

// readTestFile reads a file the test itself created (the path always comes
// from t.TempDir()) and fails the test on any error.
func readTestFile(t *testing.T, path string) []byte {
	t.Helper()

	// #nosec G304 -- the path is a test-owned t.TempDir() path, not user input
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}

// pipeStdoutNoise is written by a destination to its stdout; it must never
// surface in logs or errors because destination stdout is discarded.
const pipeStdoutNoise = "DESTINATION-STDOUT-NOISE"

// pipeStderrFill is the byte an oversized-stderr destination floods stderr
// with. It is chosen so every occurrence in an error string belongs to the
// stderr capture.
const pipeStderrFill = "Z"

// newRemoteExecMachine builds a machine whose active SSH is remote, so
// ExecPipe must route the destination through the ssh binary.
func newRemoteExecMachine(t *testing.T) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "10.0.0.1"}
	require.NoError(t, client.Init("10.0.0.1", "local-host", nixver.Info{}))

	mach.SSH = *client
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: true})

	return mach
}

// stubSSHEnvDir writes the given ssh stub script into a fresh temp dir and
// installs the record paths plus PATH for the current test, mirroring the
// PATH-stub pattern in internal/workflow/phaseops/hardwareconfig_test.go.
// t.Setenv restores PATH and the record paths after the test.
func stubSSHEnvDir(t *testing.T, script string) (string, string) {
	t.Helper()

	dir := t.TempDir()
	argvPath := filepath.Join(dir, "ssh-argv.txt")
	stdinPath := filepath.Join(dir, "ssh-stdin.bin")

	stubPath := filepath.Join(dir, "ssh")
	//nolint:gosec // the stub must be executable to be found on PATH
	require.NoError(t, os.WriteFile(stubPath, []byte(script), 0o700))

	t.Setenv("PIPE_SSH_ARGV_FILE", argvPath)
	t.Setenv("PIPE_SSH_STDIN_FILE", stdinPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return argvPath, stdinPath
}

// stubSSHInvocations writes a fake ssh that appends a delimiter line followed
// by each invocation's argv and copies stdin to a file: every invocation is
// recorded, and the stdin file is rewritten by the last one that consumes it.
func stubSSHInvocations(t *testing.T) (string, string) {
	t.Helper()

	return stubSSHEnvDir(t, `#!/bin/sh
{
  printf '== invocation\n'
  printf '%s\n' "$0" "$@"
} >> "$PIPE_SSH_ARGV_FILE"
cat > "$PIPE_SSH_STDIN_FILE"
`)
}

// sshStubInvocations splits the append-mode stub log into one argv slice per
// invocation.
func sshStubInvocations(t *testing.T, raw string) [][]string {
	t.Helper()

	blocks := strings.Split(raw, "== invocation\n")
	invocations := make([][]string, 0, len(blocks))

	for _, block := range blocks {
		block = strings.TrimRight(block, "\n")
		if block == "" {
			continue
		}

		invocations = append(invocations, strings.Split(block, "\n"))
	}

	return invocations
}

// findSSHStubInvocations returns the invocation whose joined argv contains
// probeMarker and the one containing writeMarker.
func findSSHStubInvocations(t *testing.T, invocations [][]string, probeMarker, writeMarker string) ([]string, []string) {
	t.Helper()

	var probeInvocation, writeInvocation []string

	for _, invocation := range invocations {
		joined := strings.Join(invocation, " ")

		switch {
		case strings.Contains(joined, probeMarker):
			probeInvocation = invocation
		case strings.Contains(joined, writeMarker):
			writeInvocation = invocation
		}
	}

	require.NotNil(t, probeInvocation, "probe invocation must be routed through ssh")
	require.NotNil(t, writeInvocation, "write invocation must be routed through ssh")

	return probeInvocation, writeInvocation
}

// assertSSHStubInvocation pins the ssh-stub argv shape: the ssh binary, the
// package flags, the target and the quoted destination argv at the tail.
func assertSSHStubInvocation(t *testing.T, invocation, quotedTail []string) {
	t.Helper()

	require.Greater(t, len(invocation), len(quotedTail)+3)
	assert.Equal(t, "ssh", filepath.Base(invocation[0]), "the destination must run through the ssh binary")
	assert.Equal(t, []string{"-o", "LogLevel=ERROR", "-l"}, invocation[1:4])
	assert.Contains(t, invocation, "10.0.0.1")
	assert.Equal(t, quotedTail, invocation[len(invocation)-len(quotedTail):])
}

// assertProcessGone reads a pid file written by a test process and asserts the
// pid no longer exists, proving the process was terminated and reaped.
func assertProcessGone(t *testing.T, pidPath string) {
	t.Helper()

	pid, err := strconv.Atoi(strings.TrimSpace(string(readTestFile(t, pidPath))))
	require.NoError(t, err)

	require.ErrorIs(t, syscall.Kill(pid, 0), syscall.ESRCH, "process %d must be gone", pid)
}

// assertProcessGoneWithin polls a pid file until its process disappears,
// proving the process was killed and reaped within the bound.
func assertProcessGoneWithin(t *testing.T, pidPath string, timeout time.Duration) {
	t.Helper()

	pid, err := strconv.Atoi(strings.TrimSpace(string(readTestFile(t, pidPath))))
	require.NoError(t, err)

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.Failf(t, "process still running", "process %d must be gone within %v", pid, timeout)
}

// hashProbeArgv renders a local probe argv that prints hash.
func hashProbeArgv(hash string) []string {
	return []string{"sh", "-c", "printf %s " + hash}
}

// markerHashProbeArgv renders a probe argv that records its run at markerPath
// and prints hash.
func markerHashProbeArgv(hash, markerPath string) []string {
	return []string{"sh", "-c", `printf ran > "$1"; printf %s ` + hash, "--", markerPath}
}

// payloadSHA256 returns the lowercase sha256 hex of payload.
func payloadSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:])
}

// witnessWriteArgv renders a write argv that records its execution at
// witnessPath without consuming content; absence of the witness proves the
// write was skipped.
func witnessWriteArgv(witnessPath string) []string {
	return []string{"sh", "-c", `printf wrote > "$1"`, "--", witnessPath}
}

// assertPayloadAbsentFromPhaseLog pins the hard guarantee for the phase log:
// the payload marker appears neither in CommandLog.Output nor in the JSON
// rendering.
func assertPayloadAbsentFromPhaseLog(t *testing.T, phaseLog *phaselogs.PhaseLog) {
	t.Helper()

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Zero(t, last.Output.Len())
	assert.NotContains(t, last.Output.String(), pipeSecretMarker)

	rendered, err := json.Marshal(phaseLog)
	require.NoError(t, err)
	assert.NotContains(t, string(rendered), pipeSecretMarker)
}

func TestExecPipe_BinaryExactRoundtrip(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	destinationPath := filepath.Join(t.TempDir(), "payload.bin")

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.NoError(t, err)

	assert.Equal(t, payload, readTestFile(t, destinationPath), "payload must survive the pipe byte for byte")

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Zero(t, last.Output.Len(), "secret stdout must never reach CommandLog.Output")
	assert.NotContains(t, last.Output.String(), pipeSecretMarker)
	assert.Equal(t, "sh -c "+printfScript(payload), last.Command.String(), "source argv is the logged command")
}

func TestExecPipe_SourceFailureIncludesStderr(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	destinationPath := filepath.Join(t.TempDir(), "partial.bin")
	sourceScript := `printf 'partial-output'; printf 'source-boom' >&2; exit 3`

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", sourceScript},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.Error(t, err)

	require.ErrorContains(t, err, "secret stream failed")
	require.ErrorContains(t, err, "source command failed")
	require.ErrorContains(t, err, "exit status 3")
	require.ErrorContains(t, err, "source-boom")

	assert.NoFileExists(t, destinationPath, "incomplete source content must never reach the write")
}

func TestExecPipe_DestinationFailurePropagates(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", `printf 'discarded-payload'`},
			Probe:  []string{"sh", "-c", "printf 'different-hash'"},
			Write:  []string{"sh", "-c", `printf 'destination-boom' >&2; exit 7`},
		},
	)
	require.Error(t, err)

	require.ErrorContains(t, err, "secret stream failed")
	require.ErrorContains(t, err, "destination command failed")
	require.ErrorContains(t, err, "exit status 7")
	require.ErrorContains(t, err, "destination-boom")
}

func TestExecPipe_DryRunRequiresOnDryRunWithOnSuccess(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t), func(conf *ExecutionerConf) {
		conf.DryRun = true
	})

	err := exc.ExecPipe(
		"dry secret",
		"would stream secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", "printf should-not-run"},
			Probe:  []string{"sh", "-c", "true"},
			Write:  []string{"sh", "-c", "cat"},
		},
		OnSuccess(func(*command.CommandLog) error { return nil }),
	)
	require.ErrorContains(t, err, "OnDryRun is mandatory")
	assert.Zero(t, phaseLog.CommandLogs.Length(), "validation must fail before any command is recorded")
}

func TestExecPipe_OnSuccessAndOnFailureHooks(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))
	destinationPath := filepath.Join(t.TempDir(), "out.bin")

	successCalled := false

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", `printf 'ok'`},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  writeToFileScript(destinationPath),
		},
		OnSuccess(func(*command.CommandLog) error {
			successCalled = true

			return nil
		}),
		OnDryRun(func() {}),
	)
	require.NoError(t, err)
	assert.True(t, successCalled)

	failureHookCalled := false
	hookErr := errors.New("handled failure")

	err = exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", "exit 5"},
			Probe:  []string{"sh", "-c", "true"},
			Write:  writeToFileScript(destinationPath),
		},
		OnFailure(func(_ *command.CommandLog, failure error) error {
			failureHookCalled = true

			assert.ErrorContains(t, failure, "exit status 5")

			return hookErr
		}),
		OnDryRun(func() {}),
	)
	require.ErrorIs(t, err, hookErr)
	assert.True(t, failureHookCalled)
}

func TestExecPipe_SecretNeverEntersLogsOrErrors(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	destinationPath := filepath.Join(t.TempDir(), "secret.bin")
	sourceScript := printfScript(secretPayload()) + `; printf 'source-boom' >&2; exit 4`

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", sourceScript},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "source-boom")
	assert.NotContains(t, err.Error(), pipeSecretMarker)

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.NotContains(t, last.Output.String(), pipeSecretMarker)
	assert.NotContains(t, last.Command.String(), pipeSecretMarker)
	assert.Zero(t, last.Output.Len())

	assert.NoFileExists(t, destinationPath, "a failed source must not reach the write")
}

func TestPipeCommands(t *testing.T) {
	t.Parallel()

	localClient := &ssh.SSHClient{Hostname: "local-host"}
	require.NoError(t, localClient.Init("local-host", "local-host", nixver.Info{}))

	remoteClient := &ssh.SSHClient{Hostname: "10.0.0.1", Username: "deploy", Port: 22}
	require.NoError(t, remoteClient.Init("10.0.0.1", "local-host", nixver.Info{}))

	destinationArgv := []string{"sh", "-c", `cat > /remote/out`}

	t.Run("local machine leaves the destination unwrapped", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, destinationArgv, pipeCommands(*localClient, destinationArgv))
	})

	t.Run("remote machine wraps the destination over ssh", func(t *testing.T) {
		t.Parallel()

		got := pipeCommands(*remoteClient, destinationArgv)

		require.Greater(t, len(got), len(destinationArgv))
		assert.Equal(t, []string{"ssh", "-o", "LogLevel=ERROR", "-l", "deploy", "-p", "22"}, got[:7])
		assert.Contains(t, got, "10.0.0.1")
		assert.Equal(t, []string{`'sh'`, `'-c'`, `'cat > /remote/out'`}, got[len(got)-3:])
	})
}

// TestExecPipe_DestinationStdoutDiscarded pins that destination stdout is
// diagnostics noise: it reaches neither CommandLog.Output, nor errors, nor the
// serialized phase log (nil write stdout means the null device).
func TestExecPipe_DestinationStdoutDiscarded(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	destinationPath := filepath.Join(t.TempDir(), "destination.bin")
	destinationScript := "cat > \"$1\"; printf '" + pipeStdoutNoise + "'; exit 9"

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  []string{"sh", "-c", destinationScript, "--", destinationPath},
		},
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "destination command failed")
	require.ErrorContains(t, err, "exit status 9")
	assert.NotContains(t, err.Error(), pipeStdoutNoise, "destination stdout must never reach errors")
	assert.Equal(t, payload, readTestFile(t, destinationPath), "the payload must still arrive intact")

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Zero(t, last.Output.Len())
	assert.NotContains(t, last.Output.String(), pipeStdoutNoise)

	rendered, marshalErr := json.Marshal(phaseLog)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(rendered), pipeStdoutNoise, "destination stdout must never reach the phase log rendering")
}

// TestExecPipe_OversizedDestinationStderrIsBounded pins that a destination
// flooding stderr cannot grow the error beyond the capture limit and that the
// capture stays trimmed.
func TestExecPipe_OversizedDestinationStderrIsBounded(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	destinationPath := filepath.Join(t.TempDir(), "destination.bin")
	destinationScript := `cat > "$1"; head -c 70000 /dev/zero | tr '\000' '` + pipeStderrFill + `' >&2; exit 6`

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", `printf 'payload'`},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  []string{"sh", "-c", destinationScript, "--", destinationPath},
		},
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "destination command failed")
	require.ErrorContains(t, err, "exit status 6")

	assert.Equal(t, pipeStderrCaptureLimit, strings.Count(err.Error(), pipeStderrFill),
		"captured stderr must be capped at exactly the limit")
	assert.Less(t, len(err.Error()), pipeStderrCaptureLimit+1024, "the error must stay bounded")
}

// TestExecPipe_BothFailuresReported pins that two failed processes are both
// named with their trimmed stderr excerpts, source first, on the streaming
// path; the destination drains fully before exiting so the source can finish.
func TestExecPipe_BothFailuresReported(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	sourceScript := fmt.Sprintf(`printf 'source-boom\n\n' >&2; head -c %d /dev/zero; exit 2`, pipeBufferLimit+65536)

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", sourceScript},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  []string{"sh", "-c", `printf 'destination-boom\n\n' >&2; cat > /dev/null; exit 5`},
		},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPipeCommandFailed)
	require.ErrorContains(t, err, "source command failed")
	require.ErrorContains(t, err, "exit status 2")
	require.ErrorContains(t, err, "source-boom")
	require.ErrorContains(t, err, "destination command failed")
	require.ErrorContains(t, err, "exit status 5")
	require.ErrorContains(t, err, "destination-boom")

	assert.Contains(t, err.Error(), "(stderr: source-boom)", "source stderr capture must be trimmed")
	assert.Contains(t, err.Error(), "(stderr: destination-boom)", "destination stderr capture must be trimmed")
	assert.Less(t, strings.Index(err.Error(), "source command failed"), strings.Index(err.Error(), "destination command failed"),
		"failures keep source-then-destination order")
}

// TestExecPipe_TimeoutTerminatesProcesses pins that the derived context
// terminates a hanging source during collect and a hanging destination during
// stream, with every started process reaped and the context failure named.
func TestExecPipe_TimeoutTerminatesProcesses(t *testing.T) {
	t.Parallel()

	t.Run("source hangs during collect", func(t *testing.T) {
		t.Parallel()

		assertTimeoutSourceHang(t)
	})

	t.Run("destination hangs without draining during stream", func(t *testing.T) {
		t.Parallel()

		assertTimeoutDestinationHang(t)
	})
}

// assertTimeoutSourceHang pins that a source hanging during collect is killed
// by the context before the destination can start.
func assertTimeoutSourceHang(t *testing.T) {
	t.Helper()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t), func(conf *ExecutionerConf) {
		conf.Timeout = time.Second
	})

	sourcePidPath := filepath.Join(t.TempDir(), "source.pid")
	destinationPidPath := filepath.Join(t.TempDir(), "destination.pid")

	start := time.Now()

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", `echo $$ > "$1"; exec sleep 30`, "--", sourcePidPath},
			Probe:  hashProbeArgv("unused"),
			Write:  []string{"sh", "-c", `echo $$ > "$1"`, "--", destinationPidPath},
		},
	)

	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 15*time.Second, "the timeout must terminate the hanging source")
	require.ErrorContains(t, err, "context deadline exceeded")
	require.ErrorContains(t, err, "source command failed")

	assertProcessGone(t, sourcePidPath)
	assert.NoFileExists(t, destinationPidPath, "a hanging source must never reach the destination")
}

// assertTimeoutDestinationHang pins that a destination hanging without
// draining is killed together with the blocked source.
func assertTimeoutDestinationHang(t *testing.T) {
	t.Helper()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t), func(conf *ExecutionerConf) {
		conf.Timeout = time.Second
	})

	sourcePidPath := filepath.Join(t.TempDir(), "source.pid")
	destinationPidPath := filepath.Join(t.TempDir(), "destination.pid")

	headScript := fmt.Sprintf(`head -c %d /dev/zero`, pipeBufferLimit+65536)

	start := time.Now()

	err := exc.ExecPipe(
		"stream secret",
		"streaming secret",
		"secret stream failed",
		PipeSpec{
			Source: []string{"sh", "-c", `echo $$ > "$1"; ` + headScript + `; exec sleep 30`, "--", sourcePidPath},
			Probe:  hashProbeArgv("unused"),
			Write:  []string{"sh", "-c", `echo $$ > "$1"; exec sleep 30`, "--", destinationPidPath},
		},
	)

	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 15*time.Second, "the timeout must terminate both hanging processes")
	require.ErrorContains(t, err, "context deadline exceeded")
	require.ErrorContains(t, err, "source command failed")
	require.ErrorContains(t, err, "destination command failed")

	assertProcessGone(t, sourcePidPath)
	assertProcessGone(t, destinationPidPath)
}

// TestExecPipe_RemoteWriteRouting pins that probed payloads cross the ssh
// boundary: on a remote machine both the probe and the write must run through
// the ssh binary and the payload must reach the write's stdin. The overflow
// subtest proves the same for the streaming path, where the probe is skipped
// by design so only the write is invoked.
//
//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestExecPipe_RemoteWriteRouting(t *testing.T) {
	t.Run("complete-path write", func(t *testing.T) {
		assertExecPipeRemoteCompleteWrite(t)
	})

	t.Run("overflow write", func(t *testing.T) {
		assertExecPipeRemoteOverflowWrite(t)
	})
}

// assertExecPipeRemoteCompleteWrite drives the buffered path with a remote
// machine: the probe reports a mismatch (empty stub output) and both the probe
// and the write must reach the ssh stub.
func assertExecPipeRemoteCompleteWrite(t *testing.T) {
	t.Helper()

	argvPath, stdinPath := stubSSHInvocations(t)

	exc, phaseLog := newLocalExecutioner(t, newRemoteExecMachine(t))

	payload := secretPayload()

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  []string{"sh", "-c", "printf probe-hash"},
			Write:  []string{"sh", "-c", `cat > "$1"`, "--", "/remote/payload.bin"},
		},
	)
	require.NoError(t, err)

	assert.Equal(t, payload, readTestFile(t, stdinPath), "the ssh stub must receive the payload byte-exact")

	invocations := sshStubInvocations(t, string(readTestFile(t, argvPath)))
	require.Len(t, invocations, 2)

	probeInvocation, writeInvocation := findSSHStubInvocations(t, invocations, "probe-hash", "cat >")
	assertSSHStubInvocation(t, probeInvocation, []string{`'sh'`, `'-c'`, `'printf probe-hash'`})
	assertSSHStubInvocation(t, writeInvocation,
		[]string{`'sh'`, `'-c'`, `'cat > "$1"'`, `'--'`, `'/remote/payload.bin'`})

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Equal(t, "sh -c "+printfScript(payload), last.Command.String(), "the logged command stays the source argv")
}

// assertExecPipeRemoteOverflowWrite drives the streaming path with a remote
// machine: content beyond pipeBufferLimit skips the probe, and the streamed
// payload must reach the write through the ssh stub.
func assertExecPipeRemoteOverflowWrite(t *testing.T) {
	t.Helper()

	argvPath, stdinPath := stubSSHInvocations(t)

	exc, phaseLog := newLocalExecutioner(t, newRemoteExecMachine(t))

	payload := bytes.Repeat([]byte{0x00}, pipeBufferLimit+4096)

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", fmt.Sprintf("head -c %d /dev/zero", pipeBufferLimit+4096)},
			Probe:  []string{"sh", "-c", "printf probe-hash"},
			Write:  []string{"sh", "-c", `cat > "$1"`, "--", "/remote/payload.bin"},
		},
	)
	require.NoError(t, err)

	assert.Equal(t, payload, readTestFile(t, stdinPath), "the ssh stub must receive the streamed payload byte-exact")

	invocations := sshStubInvocations(t, string(readTestFile(t, argvPath)))
	require.Len(t, invocations, 1, "overflow commits to the write without probing")

	assertSSHStubInvocation(t, invocations[0],
		[]string{`'sh'`, `'-c'`, `'cat > "$1"'`, `'--'`, `'/remote/payload.bin'`})

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Equal(t, "sh -c "+fmt.Sprintf("head -c %d /dev/zero", pipeBufferLimit+4096),
		last.Command.String(), "the logged command stays the source argv")
}

// TestExecPipe_SkipsUnchangedContent pins the skip path: a probe hash matching
// the freshly produced content skips the write entirely.
func TestExecPipe_SkipsUnchangedContent(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	probeMarker := filepath.Join(t.TempDir(), "probe-ran")
	witness := filepath.Join(t.TempDir(), "write-ran")

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  markerHashProbeArgv(payloadSHA256(payload), probeMarker),
			Write:  witnessWriteArgv(witness),
		},
	)
	require.NoError(t, err)

	assert.Equal(t, "ran", string(readTestFile(t, probeMarker)), "the probe must have run")
	assert.NoFileExists(t, witness, "the write must be skipped when the hashes match")

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Equal(t, "sh -c "+printfScript(payload), last.Command.String())
}

// TestExecPipe_WritesChangedContent pins the update path with a binary
// payload: a probe hash that differs from the produced content forces the
// write, byte for byte.
func TestExecPipe_WritesChangedContent(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	destinationPath := filepath.Join(t.TempDir(), "secret.bin")

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.NoError(t, err)
	assert.Equal(t, payload, readTestFile(t, destinationPath), "changed content must be written byte for byte")
}

// TestExecPipe_WritesWhenTargetMissing pins the missing-file contract: a probe
// that prints nothing and exits 0 means update.
func TestExecPipe_WritesWhenTargetMissing(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	destinationPath := filepath.Join(t.TempDir(), "secret.bin")

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  []string{"sh", "-c", "true"},
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.NoError(t, err)
	assert.Equal(t, payload, readTestFile(t, destinationPath))
}

// TestExecPipe_WritesWhenProbeFails pins that a failing probe degrades
// silently to a normal write and never surfaces on its own.
func TestExecPipe_WritesWhenProbeFails(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	destinationPath := filepath.Join(t.TempDir(), "secret.bin")

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  []string{"sh", "-c", "printf 'probe-boom' >&2; exit 3"},
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.NoError(t, err, "probe failures must never surface on their own")
	assert.Equal(t, payload, readTestFile(t, destinationPath))
}

// TestExecPipe_WritesWhenProbeOutputUnparseable pins that output which is not
// a sha256 hex means update.
func TestExecPipe_WritesWhenProbeOutputUnparseable(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	destinationPath := filepath.Join(t.TempDir(), "secret.bin")

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", printfScript(payload)},
			Probe:  []string{"sh", "-c", "printf 'not-a-hash'"},
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.NoError(t, err)
	assert.Equal(t, payload, readTestFile(t, destinationPath))
}

// TestExecPipe_BufferBoundaryStaysComplete pins the exact boundary: content of
// exactly pipeBufferLimit bytes counts as complete, not as overflow, so a
// matching probe still skips the write.
func TestExecPipe_BufferBoundaryStaysComplete(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := bytes.Repeat([]byte{0x00}, pipeBufferLimit)
	witness := filepath.Join(t.TempDir(), "write-ran")

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", fmt.Sprintf("head -c %d /dev/zero", pipeBufferLimit)},
			Probe:  hashProbeArgv(payloadSHA256(payload)),
			Write:  witnessWriteArgv(witness),
		},
	)
	require.NoError(t, err)
	assert.NoFileExists(t, witness, "content of exactly pipeBufferLimit bytes must take the complete path and skip")
}

// TestExecPipe_OverflowStreamsAndRunsSourceOnce pins the overflow path:
// content beyond pipeBufferLimit streams straight to the write, the probe is
// not run, and the source executes exactly once.
func TestExecPipe_OverflowStreamsAndRunsSourceOnce(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := bytes.Repeat([]byte{0x00}, pipeBufferLimit+4096)
	destinationPath := filepath.Join(t.TempDir(), "secret.bin")
	runCountPath := filepath.Join(t.TempDir(), "source-runs")
	probeMarker := filepath.Join(t.TempDir(), "probe-ran")

	sourceScript := fmt.Sprintf(`printf 'run\n' >> "$1"; head -c %d /dev/zero`, pipeBufferLimit+4096)

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", sourceScript, "--", runCountPath},
			Probe:  []string{"sh", "-c", `printf ran > "$1"`, "--", probeMarker},
			Write:  writeToFileScript(destinationPath),
		},
	)
	require.NoError(t, err)

	assert.Equal(t, payload, readTestFile(t, destinationPath), "streamed content must arrive byte for byte")
	assert.Equal(t, "run\n", string(readTestFile(t, runCountPath)), "the source must execute exactly once")
	assert.NoFileExists(t, probeMarker, "overflow commits to the write without probing")
}

// TestExecPipe_DryRunStartsNothing pins the dry-run contract: the source argv
// is recorded, nothing starts (no probe, no write) and OnDryRun runs.
func TestExecPipe_DryRunStartsNothing(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t), func(conf *ExecutionerConf) {
		conf.DryRun = true
	})

	probeMarker := filepath.Join(t.TempDir(), "probe-ran")
	witness := filepath.Join(t.TempDir(), "write-ran")
	dryRunCalled := false

	err := exc.ExecPipe(
		"dry update",
		"would update secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", "printf should-not-run"},
			Probe:  markerHashProbeArgv(payloadSHA256([]byte("irrelevant")), probeMarker),
			Write:  witnessWriteArgv(witness),
		},
		OnDryRun(func() { dryRunCalled = true }),
	)
	require.NoError(t, err)
	assert.True(t, dryRunCalled)

	assert.NoFileExists(t, probeMarker, "dry run must not start the probe")
	assert.NoFileExists(t, witness, "dry run must not start the write")

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Equal(t, "sh -c printf should-not-run", last.Command.String())
}

// TestExecPipe_PayloadNeverLogged pins the hard guarantee on both the
// successful write path and the failed write path.
func TestExecPipe_PayloadNeverLogged(t *testing.T) {
	t.Parallel()

	payload := secretPayload()

	t.Run("successful write", func(t *testing.T) {
		t.Parallel()

		exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

		destinationPath := filepath.Join(t.TempDir(), "secret.bin")

		err := exc.ExecPipe(
			"update secret",
			"updating secret",
			"secret update failed",
			PipeSpec{
				Source: []string{"sh", "-c", printfScript(payload)},
				Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
				Write:  writeToFileScript(destinationPath),
			},
		)
		require.NoError(t, err)
		assert.Equal(t, payload, readTestFile(t, destinationPath))
		assertPayloadAbsentFromPhaseLog(t, phaseLog)
	})

	t.Run("failed write", func(t *testing.T) {
		t.Parallel()

		exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

		err := exc.ExecPipe(
			"update secret",
			"updating secret",
			"secret update failed",
			PipeSpec{
				Source: []string{"sh", "-c", printfScript(payload)},
				Probe:  hashProbeArgv(payloadSHA256([]byte("stale content"))),
				Write:  []string{"sh", "-c", "printf 'write-boom' >&2; exit 8"},
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "write-boom")
		assert.NotContains(t, err.Error(), pipeSecretMarker)

		assertPayloadAbsentFromPhaseLog(t, phaseLog)
	})
}

// TestExecPipe_RejectsInvalidSpec pins that an empty argv is rejected before
// prepareCommand can index it and before any CommandLog entry exists.
func TestExecPipe_RejectsInvalidSpec(t *testing.T) {
	t.Parallel()

	source := []string{"sh", "-c", "printf secret"}
	probe := []string{"sh", "-c", "true"}
	write := []string{"sh", "-c", "cat"}

	tests := []struct {
		name    string
		spec    PipeSpec
		wantErr string
	}{
		{name: "empty source", spec: PipeSpec{Probe: probe, Write: write}, wantErr: "source"},
		{name: "empty probe", spec: PipeSpec{Source: source, Write: write}, wantErr: "probe"},
		{name: "empty write", spec: PipeSpec{Source: source, Probe: probe}, wantErr: "write"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

			err := exc.ExecPipe("bad spec", "bad spec", "bad spec failed", tt.spec)
			require.ErrorContains(t, err, tt.wantErr)
			assert.Zero(t, phaseLog.CommandLogs.Length(), "an invalid spec must not create a CommandLog entry")
		})
	}
}

// TestExecPipe_RejectsUnsupportedOptions pins that options ExecPipe cannot
// honor fail loudly instead of being ignored.
func TestExecPipe_RejectsUnsupportedOptions(t *testing.T) {
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
		{name: "SkipIfLocal", opt: SkipIfLocal(), wantErr: "SkipIfLocal"},
		{name: "DisableAutoSSHCommand", opt: DisableAutoSSHCommand(), wantErr: "DisableAutoSSHCommand"},
		{name: "Trim", opt: Trim(), wantErr: "Trim"},
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

// TestPipeRun_FailureContextHandling pins the context fold: an expired context
// joins a process failure but never turns a completed operation into a
// failure.
func TestPipeRun_FailureContextHandling(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	run := &pipeRun{ctx: ctx, sourceErr: new(boundedCapture)}

	require.NoError(t, run.failure(nil, nil, ""), "a completed operation must not report a context failure")

	err := run.failure(errors.New("boom"), nil, "")
	require.ErrorContains(t, err, "source command failed: boom")
	require.ErrorContains(t, err, "context: context canceled")
}

// TestExecPipe_HashesOnlyAfterSourceWait pins the ordering invariant: the
// payload is hashed and the probe consulted only after the source exits 0, so
// a matching probe hash cannot mask a failed source.
func TestExecPipe_HashesOnlyAfterSourceWait(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	payload := secretPayload()
	probeMarker := filepath.Join(t.TempDir(), "probe-ran")
	targetPath := filepath.Join(t.TempDir(), "secret.bin")

	sourceScript := printfScript(payload) + "; exit 3"

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", sourceScript},
			Probe:  markerHashProbeArgv(payloadSHA256(payload), probeMarker),
			Write:  writeToFileScript(targetPath),
		},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPipeCommandFailed)
	require.ErrorContains(t, err, "source command failed")
	require.ErrorContains(t, err, "exit status 3")

	assert.NoFileExists(t, probeMarker, "the probe must not run before a successful source wait")
	assert.NoFileExists(t, targetPath, "a failed source must never write")

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Zero(t, last.Output.Len(), "secret stdout must never reach CommandLog.Output")
}

// TestExecPipe_StreamStartFailureReapsSource pins abort on a stream
// destination-start failure: the already-live source must be killed and
// reaped, not left blocked on a pipe nobody drains.
func TestExecPipe_StreamStartFailureReapsSource(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	sourcePidPath := filepath.Join(t.TempDir(), "source.pid")
	missingWrite := filepath.Join(t.TempDir(), "missing-write-binary")

	sourceScript := fmt.Sprintf(`echo $$ > "$1"; head -c %d /dev/zero`, 64<<20)

	err := exc.ExecPipe(
		"update secret",
		"updating secret",
		"secret update failed",
		PipeSpec{
			Source: []string{"sh", "-c", sourceScript, "--", sourcePidPath},
			Probe:  []string{"sh", "-c", "true"},
			Write:  []string{missingWrite},
		},
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to start destination command")

	assertProcessGoneWithin(t, sourcePidPath, 5*time.Second)
}
