package executioner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertNoStdinPumpGoroutine polls the runtime stacks until no stdin pump
// goroutine remains, proving the pump exits after the exec ends or the
// context is canceled instead of leaking on a blocked Read. Only sequential
// tests may call it: a concurrent pump-spawning test would put its own live
// pump goroutines into the same scan. The scan matches the pump goroutine
// frame names, never the helper's own name on the scanning stack.
func assertNoStdinPumpGoroutine(t *testing.T) {
	t.Helper()

	require.Eventually(t, func() bool {
		stacks := make([]byte, 1<<20)
		stacks = stacks[:runtime.Stack(stacks, true)]

		return !bytes.Contains(stacks, []byte("pumpStdin")) &&
			!bytes.Contains(stacks, []byte("unblockStdinPumpOnCancel"))
	}, 5*time.Second, 20*time.Millisecond, "stdin pump goroutines must exit")
}

// errorWriter accepts the first write and fails every later one, so a test
// can pin that the pump stops at the first write error instead of retrying.
type errorWriter struct {
	writes int
}

func (w *errorWriter) Write(data []byte) (int, error) {
	w.writes++

	if w.writes == 1 {
		return len(data), nil
	}

	return 0, errors.New("master gone")
}

// delayedOnceReader delivers its payload after a delay on the first read and
// io.EOF afterward. It simulates a control-channel peer that answers only
// after the exec already ended.
type delayedOnceReader struct {
	delay   time.Duration
	payload string
	spent   bool
}

func (r *delayedOnceReader) Read(dst []byte) (int, error) {
	if r.spent {
		return 0, io.EOF
	}

	r.spent = true
	time.Sleep(r.delay)

	return copy(dst, r.payload), nil
}

// TestStdinPump_DeliversBytesInOrderAndStopsAtEOF pins the pump core: bytes
// cross in stream order and the pump exits by itself when the reader ends.
func TestStdinPump_DeliversBytesInOrderAndStopsAtEOF(t *testing.T) {
	t.Parallel()

	dst := &bytes.Buffer{}
	src := strings.NewReader("frame-1\nframe-2\nframe-3\n")

	done := startStdinPump(context.Background(), dst, src)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the pump must exit when the reader ends")
	}

	assert.Equal(t, "frame-1\nframe-2\nframe-3\n", dst.String())
}

// TestStdinPump_CancelUnblocksBlockedRead pins the cancellation path: a pump
// blocked on a reader that will never answer is released by the context
// through the expired read deadline and exits instead of leaking.
func TestStdinPump_CancelUnblocksBlockedRead(t *testing.T) {
	t.Parallel()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	defer func() { _ = reader.Close(); _ = writer.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := startStdinPump(ctx, io.Discard, reader)

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "cancel must unblock a pump waiting on its reader")
	}
}

// TestStdinPump_WriteErrorEndsPump pins the best-effort transport contract at
// the pump level: the first master write failure ends the pump silently, with
// no retries and no further writes.
func TestStdinPump_WriteErrorEndsPump(t *testing.T) {
	t.Parallel()

	src := io.MultiReader(strings.NewReader("chunk-1\n"), strings.NewReader("chunk-2\n"))
	dst := &errorWriter{}

	done := startStdinPump(context.Background(), dst, src)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "a write error must end the pump")
	}

	assert.Equal(t, 2, dst.writes, "the pump must stop at the first write error")
}

// TestStdinPump_ReadErrorEndsPump pins that any read error, not only io.EOF,
// ends the pump instead of spinning.
func TestStdinPump_ReadErrorEndsPump(t *testing.T) {
	t.Parallel()

	src := &scriptedReader{steps: []scriptedStep{{err: errors.New("reader boom")}}}

	done := startStdinPump(context.Background(), io.Discard, src)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "a read error must end the pump")
	}
}

// TestExec_WithStdin_DeliversLinesAndPumpExitsAfterExec pins the local exec
// wiring end to end: the frames reach the command in order, the exec outcome
// is untouched by the pump, and the pump goroutine exits after the exec ends
// even though the stdin pipe stays open (the exec-end cancellation path).
func TestExec_WithStdin_DeliversLinesAndPumpExitsAfterExec(t *testing.T) {
	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	defer func() { _ = reader.Close(); _ = writer.Close() }()

	_, err = writer.WriteString("frame-1\nframe-2\n")
	require.NoError(t, err)

	err = exc.Exec(
		"stdin lines", "streaming stdin", "stdin failed",
		[]string{"sh", "-c", `IFS= read -r first; IFS= read -r second; echo "got:$first"; echo "got:$second"`},
		WithStdin(reader),
	)
	require.NoError(t, err)

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)

	out := last.Output.String()

	first := strings.Index(out, "got:frame-1")
	second := strings.Index(out, "got:frame-2")

	require.NotEqual(t, -1, first, "the first frame must reach the command")
	require.NotEqual(t, -1, second, "the second frame must reach the command")
	assert.Less(t, first, second, "frames must be delivered in stream order")

	assertNoStdinPumpGoroutine(t)
}

// TestExec_WithStdin_TimeoutCancelsPump pins the timeout path: a short
// WithTimeout fails the exec and still releases the pump blocked on its
// reader, leaving no goroutine behind.
func TestExec_WithStdin_TimeoutCancelsPump(t *testing.T) {
	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	defer func() { _ = reader.Close(); _ = writer.Close() }()

	start := time.Now()

	err = exc.Exec(
		"hanging stdin", "streaming", "failed",
		[]string{"sh", "-c", "sleep 30"},
		WithStdin(reader),
		WithTimeout(100*time.Millisecond),
	)
	assertKilledFast(t, err, start)

	assertNoStdinPumpGoroutine(t)
}

// TestExec_WithStdin_WriteErrorKeepsOutcomeClean pins the exec-level
// best-effort contract: a pump write that fails because the child already
// exited (closed master) never turns a successful exec into a failure and
// leaves no pump goroutine behind.
func TestExec_WithStdin_WriteErrorKeepsOutcomeClean(t *testing.T) {
	exc, _ := newLocalExecutioner(t, newLocalExecMachine(t))

	err := exc.Exec(
		"exiting child", "running", "failed",
		[]string{"sh", "-c", "true"},
		WithStdin(&delayedOnceReader{delay: 200 * time.Millisecond, payload: "late-payload\n"}),
	)
	require.NoError(t, err, "a late pump write error must not affect the exec outcome")

	assertNoStdinPumpGoroutine(t)
}

// TestExec_WithStdin_DisablesPTYEcho pins the auto echo-disable: the frame
// pumped into the master must not be echoed back into the output stream, so
// the inbound protocol stream carries exactly the child's own output (the
// guard's echo defense stays belt-and-braces on top). The echo flag is fixed
// before the pump starts, so the line discipline's asynchronous echo decision
// can never straddle the first frame.
func TestExec_WithStdin_DisablesPTYEcho(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	err := exc.Exec(
		"echo defense", "streaming", "failed",
		[]string{"sh", "-c", `IFS= read -r line; printf 'acked:%s\n' "$line"`},
		WithStdin(strings.NewReader("secret-frame\n")),
	)
	require.NoError(t, err)

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)

	out := last.Output.String()

	require.Contains(t, out, "acked:secret-frame", "the frame must reach the command")
	assert.Equal(t, 1, strings.Count(out, "secret-frame"),
		"the PTY echo must be off for WithStdin execs: the frame may appear only inside the child's own output, got %q", out)
}

// TestExec_WithStdin_RemoteForwardsToSSHStub mirrors the execpipe byte-exact
// stdin pattern for the streaming exec: the frames pumped into the PTY master
// reach the ssh child's stdin (the slave) and are forwarded to the remote
// side, byte for byte.
//
// The stub drains exactly len(payload) bytes with head -c and exits, because
// a PTY swallows end-of-stream: a plain `cat > file` would wait for an EOF
// that never travels through a PTY and only the timeout could end the exec.
//
//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestExec_WithStdin_RemoteForwardsToSSHStub(t *testing.T) {
	payload := "guard-frame-01\nguard-frame-02\n"

	script := fmt.Sprintf(`#!/bin/sh
{
  printf '== invocation\n'
  printf '%%s\n' "$0" "$@"
} >> "$PIPE_SSH_ARGV_FILE"
head -c %d > "$PIPE_SSH_STDIN_FILE"
`, len(payload))

	argvPath, stdinPath := stubSSHEnvDir(t, script)

	exc, _ := newLocalExecutioner(t, newRemoteExecMachine(t))

	err := exc.Exec(
		"guard control", "talking", "talk failed",
		[]string{"panix-guard", "ctl", "confirm"},
		WithStdin(strings.NewReader(payload)),
	)
	require.NoError(t, err)

	assert.Equal(t, payload, string(readTestFile(t, stdinPath)),
		"the ssh stub must receive the frames byte-exact through the PTY")

	assertNoStdinPumpGoroutine(t)

	invocations := sshStubInvocations(t, string(readTestFile(t, argvPath)))
	require.Len(t, invocations, 1)
	assert.False(t, slices.Contains(invocations[0], "-t"), "the streaming exec must not request a remote PTY")
	assert.False(t, slices.Contains(invocations[0], "-tt"))
}

// TestSSHCommandWithArgs_NoRemotePTYRequest pins the transport shape the
// duplex control channel relies on: the streaming ssh argv never contains
// -t or -tt, so no remote PTY is requested and the remote side stays
// non-interactive (see the sshStream elevation note).
func TestSSHCommandWithArgs_NoRemotePTYRequest(t *testing.T) {
	t.Parallel()

	client := ssh.SSHClient{Hostname: "10.0.0.1", Port: 22, Username: "deploy"}

	tests := []struct {
		name string
		opts sshArgvOptions
	}{
		{name: "default multiplexed transport", opts: sshArgvOptions{}},
		{name: "fresh connection transport", opts: sshArgvOptions{freshConnection: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := sshCommandWithArgsOptions(client, []string{"panix-guard", "ctl", "confirm"}, tt.opts)

			assert.False(t, slices.Contains(got, "-t"), "a remote PTY request would rewrap and double-echo the stream")
			assert.False(t, slices.Contains(got, "-tt"))
		})
	}
}

// TestExecPipe_RejectsWithStdinAndOutputTap pins that the Exec-only duplex
// options fail loudly on the pipe path instead of being ignored.
func TestExecPipe_RejectsWithStdinAndOutputTap(t *testing.T) {
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
		{name: "WithStdin", opt: WithStdin(strings.NewReader("x")), wantErr: "WithStdin"},
		{name: "WithOutputTap", opt: WithOutputTap(func([]byte) {}), wantErr: "WithOutputTap"},
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

// TestExecOptions_StdinAndTapDefaultToUnset pins the zero-value contract:
// without the new options the pump never starts and the tap is never invoked,
// which is the zero behavior change guarantee for every existing caller.
func TestExecOptions_StdinAndTapDefaultToUnset(t *testing.T) {
	t.Parallel()

	excOpt := &ExecOptions{}
	assert.Nil(t, excOpt.stdin, "without WithStdin the pump must not start")
	assert.Nil(t, excOpt.outputTap, "without WithOutputTap the tap must not be invoked")

	WithStdin(strings.NewReader("x"))(excOpt)
	WithOutputTap(func([]byte) {})(excOpt)
	assert.NotNil(t, excOpt.stdin)
	assert.NotNil(t, excOpt.outputTap)
}
