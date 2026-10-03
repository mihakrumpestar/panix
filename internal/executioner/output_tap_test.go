package executioner

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExec_WithOutputTap_DeliversRawChunksInOrder pins the tap wiring end to
// end: the callback receives the raw PTY chunks in stream order while the
// CommandLog rendering path stays completely unaffected.
func TestExec_WithOutputTap_DeliversRawChunksInOrder(t *testing.T) {
	t.Parallel()

	exc, phaseLog := newLocalExecutioner(t, newLocalExecMachine(t))

	var chunks [][]byte

	err := exc.Exec(
		"tapped output", "streaming", "failed",
		[]string{"sh", "-c", `printf 'tap-alpha'; sleep 0.05; printf 'tap-beta'`},
		WithOutputTap(func(chunk []byte) { chunks = append(chunks, bytes.Clone(chunk)) }),
	)
	require.NoError(t, err)

	assert.NotEmpty(t, chunks, "the tap must observe the output stream")
	assert.Equal(t, "tap-alphatap-beta", string(bytes.Join(chunks, nil)),
		"tap chunks must preserve the raw stream order")

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Contains(t, last.Output.String(), "tap-alpha", "the CommandLog rendering path stays unaffected")
	assert.Contains(t, last.Output.String(), "tap-beta")
}

// TestReadPTYOutputTap_ChunkOrderAndNilTap pins the read-loop contract at the
// unit level: the tap sees raw chunks before terminal processing, in stream
// order, and a nil tap keeps the plain readPTYOutput behavior.
func TestReadPTYOutputTap_ChunkOrderAndNilTap(t *testing.T) {
	t.Parallel()

	t.Run("tap receives raw chunks in stream order", func(t *testing.T) {
		t.Parallel()

		ex := NewExecutioner(ExecutionerConf{Ctx: t.Context(), OnUpdateHook: func() {}})
		cmdLog := newTestCommandLog()

		var got []string

		err := ex.readPTYOutputTap(ex.conf.Ctx, &scriptedReader{steps: []scriptedStep{
			{data: "chunk-1"},
			{data: "chunk-2\r\n"},
			{err: io.EOF},
		}}, cmdLog, func(chunk []byte) { got = append(got, string(chunk)) })
		require.NoError(t, err)

		assert.Equal(t, []string{"chunk-1", "chunk-2\r\n"}, got,
			"the tap must see raw chunks, not terminal-processed lines")
		assertLines(t, cmdLog, []string{"chunk-1chunk-2"})
	})

	t.Run("nil tap keeps the plain behavior", func(t *testing.T) {
		t.Parallel()

		ex := NewExecutioner(ExecutionerConf{Ctx: t.Context(), OnUpdateHook: func() {}})
		cmdLog := newTestCommandLog()

		err := ex.readPTYOutputTap(ex.conf.Ctx, &scriptedReader{steps: []scriptedStep{
			{data: "plain\r\n"},
			{err: io.EOF},
		}}, cmdLog, nil)
		require.NoError(t, err)

		assertLines(t, cmdLog, []string{"plain"})
	})
}
