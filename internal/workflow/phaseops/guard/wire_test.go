package guard

import (
	"strings"
	"sync"
	"testing"

	"github.com/mihakrumpestar/panix/internal/guard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wireRec builds one valid record line for the wire test deploy key. The
// timestamp is a fixed epoch: nothing in these tests depends on its value.
func wireRec(seq uint64, event string, r guard.Record) string {
	r.K, r.W, r.Seq, r.TS, r.Ev = "wire-key", guard.WriterGuardian, seq, 1700000000, event

	return guard.FormatRecord(r)
}

// TestWireStream_ReassemblesSplitChunks pins the tap contract end to end:
// records split across PTY chunk boundaries (with ONLCR line endings)
// reassemble into exactly one parsed record.
func TestWireStream_ReassemblesSplitChunks(t *testing.T) {
	t.Parallel()

	stream := newWireStream("wire-key")

	record := wireRec(1, guard.EventHello, guard.Record{PID: 4242, Old: testOld, New: testNew, Gen: 7})
	for _, chunk := range []string{record[:10], record[10:25], record[25:] + "\r\n"} {
		stream.feed([]byte(chunk))
	}

	records, cursor := stream.drain(0)

	require.Len(t, records, 1, "one record line, no matter how the chunks split it")
	assert.Equal(t, guard.EventHello, records[0].Ev)
	assert.Equal(t, 4242, records[0].PID)
	assert.Equal(t, testOld, records[0].Old)
	assert.Equal(t, int64(7), records[0].Gen)
	assert.Equal(t, 1, cursor, "the high-water mark advances past the record")

	next, nextCursor := stream.drain(cursor)
	assert.Empty(t, next)
	assert.Equal(t, cursor, nextCursor, "an up-to-date cursor drains nothing")
}

// TestWireStream_EchoDefenseAndNarrative pins the line classification: echoed
// command frames never become events, narrative stays out of the record path,
// and foreign-key or malformed sentinel lines are noise.
func TestWireStream_EchoDefenseAndNarrative(t *testing.T) {
	t.Parallel()

	stream := newWireStream("wire-key")

	stream.feed([]byte("{\"c\":\"confirm\",\"rid\":1}\r\n"))                       // echoed frame
	stream.feed([]byte("free-form activation output\nanother narrative line\r\n")) // narrative
	stream.feed([]byte(wireRec(1, guard.EventActivated, guard.Record{}) + "\r\n")) // own record
	stream.feed([]byte(wireRec(2, guard.EventCommitted, guard.Record{St: "committed"}) + "\r\n"))

	foreign := strings.Replace(wireRec(3, guard.EventState, guard.Record{St: "activating"}), "wire-key", "other-key", 1)
	stream.feed([]byte(foreign + "\n"))
	stream.feed([]byte("@PG2 not json\n"))

	records, _ := stream.drain(0)

	require.Len(t, records, 2, "only the deploy-key records pass the gate")
	assert.Equal(t, guard.EventActivated, records[0].Ev)
	assert.Equal(t, guard.EventCommitted, records[1].Ev)
}

// TestWireStream_SnapshotFoldsContext pins the streamed context: HELLO and
// TXN records carry old/new/gen and the snapshot folds the newest values.
// The TXN line is spelled literally because the event name lands with the
// guardian-side lane; the parser accepts any event string.
func TestWireStream_SnapshotFoldsContext(t *testing.T) {
	t.Parallel()

	stream := newWireStream("wire-key")

	txn := `@PG2 {"v":1,"k":"wire-key","w":"g","seq":2,"ts":102,"ev":"TXN","old":"` + testOld + `","new":"` + testNew + `","gen":3}`

	stream.feed([]byte(wireRec(1, guard.EventHello, guard.Record{PID: 7, Old: testOld, New: testNew, Gen: 3}) + "\n"))
	stream.feed([]byte(txn + "\n"))
	stream.feed([]byte(wireRec(3, guard.EventState, guard.Record{St: "activating"}) + "\n"))

	snapshot := stream.snapshot()

	assert.True(t, snapshot.Found)
	assert.Equal(t, testOld, snapshot.Old)
	assert.Equal(t, testNew, snapshot.New)
	assert.Equal(t, int64(3), snapshot.Gen)
	assert.Equal(t, "activating", snapshot.Status)
	assert.False(t, snapshot.Terminal)
}

// TestWireStream_IngestTailFeedsTheFloor pins the floor path: tail-read
// records join the same stream and cursor, so the loop's decisions are
// transport-agnostic.
func TestWireStream_IngestTailFeedsTheFloor(t *testing.T) {
	t.Parallel()

	stream := newWireStream("wire-key")

	_, cursor := stream.drain(0)

	stream.ingestTail(strings.Join([]string{
		wireRec(1, guard.EventState, guard.Record{St: "activating"}),
		wireRec(2, guard.EventActivated, guard.Record{}),
	}, "\n"))

	records, next := stream.drain(cursor)

	require.Len(t, records, 2)
	assert.Equal(t, guard.EventActivated, records[1].Ev)
	assert.Equal(t, 2, next)
}

// TestWireStream_CapCompactionKeepsCursorConsistent pins the bounded buffer:
// beyond the cap the oldest records compact away and the cursor mapping stays
// monotonic (a stale cursor re-drives from the oldest retained record).
func TestWireStream_CapCompactionKeepsCursorConsistent(t *testing.T) {
	t.Parallel()

	stream := newWireStream("wire-key")

	for i := range streamRecordCap + 5 {
		stream.feed([]byte(wireRec(uint64(i+1), guard.EventState, guard.Record{St: "activating"}) + "\n"))
	}

	records, cursor := stream.drain(0)

	assert.Len(t, records, streamRecordCap, "the buffer stays bounded")
	assert.Equal(t, streamRecordCap+5, cursor)

	// A cursor from before the compaction still yields the retained window.
	records, cursor = stream.drain(1)
	assert.Len(t, records, streamRecordCap)
	assert.Equal(t, streamRecordCap+5, cursor)

	_, cursor = stream.drain(cursor)
	empty, _ := stream.drain(cursor)
	assert.Empty(t, empty)
}

// TestWireStream_ConcurrentFeedAndDrain pins the must-not-block contract
// under the race detector: the tap feed and the loop's drain run concurrently
// without torn records or lost wakeups.
func TestWireStream_ConcurrentFeedAndDrain(t *testing.T) {
	t.Parallel()

	stream := newWireStream("wire-key")

	var wg sync.WaitGroup

	wg.Go(func() {
		for i := range 200 {
			stream.feed([]byte(wireRec(uint64(i+1), guard.EventState, guard.Record{St: "activating"}) + "\n"))
		}
	})

	seen := 0

	for seen < 200 {
		<-stream.notify

		records, _ := stream.drain(0)
		seen = len(records)
	}

	wg.Wait()

	records, _ := stream.drain(0)
	assert.Len(t, records, 200, "every record survives concurrent feeding")
}

// TestFormatFrame pins the wire frame shapes (the guardian-side contract):
// one compact JSON object per line, newline-terminated.
func TestFormatFrame(t *testing.T) {
	t.Parallel()

	assert.JSONEq(t, "{\"c\":\"confirm\",\"rid\":1}\n", formatFrame(frameConfirm, 1))
	assert.JSONEq(t, "{\"c\":\"revert\",\"rid\":2}\n", formatFrame(frameRevert, 2))
}

// TestIsCommandFrame pins the echo-defense predicate: command frame shapes
// match, everything else (records, narrative, broken JSON) does not.
func TestIsCommandFrame(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "confirm frame", line: `{"c":"confirm","rid":1}`, want: true},
		{name: "revert frame", line: `{"c":"revert","rid":7}`, want: true},
		{name: "padded frame", line: `  {"c":"confirm","rid":1}  `, want: true},
		{name: "frame without verb", line: `{"rid":1}`, want: false},
		{name: "record line", line: wireRec(1, guard.EventState, guard.Record{}), want: false},
		{name: "narrative", line: "activation output", want: false},
		{name: "broken json", line: `{"c":`, want: false},
		{name: "empty", line: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isCommandFrame([]byte(tt.line)))
		})
	}
}

// TestParseWireRecord pins the record gate: valid deploy-key records parse
// with their rid, everything else is noise.
func TestParseWireRecord(t *testing.T) {
	t.Parallel()

	t.Run("record with rid", func(t *testing.T) {
		t.Parallel()

		rec, ok := parseWireRecord(wireRec(1, guard.EventConfirmConsumed, guard.Record{St: "confirming"}), "wire-key")

		require.True(t, ok)
		assert.Equal(t, guard.EventConfirmConsumed, rec.Ev)
		assert.Equal(t, int64(0), rec.Rid, "a record without the rid field reads as rid 0")
	})

	t.Run("foreign key is noise", func(t *testing.T) {
		t.Parallel()

		line := strings.Replace(wireRec(1, guard.EventState, guard.Record{}), "wire-key", "other", 1)

		_, ok := parseWireRecord(line, "wire-key")
		assert.False(t, ok)
	})
}

// TestIsWireAck pins the ack correlation rules (the guardian-side contract):
// the matching rid gates every ack, the four ack events map per request kind,
// and the status-late ack counts for either.
func TestIsWireAck(t *testing.T) {
	t.Parallel()

	rec := func(rid int64, event, st string) streamRecord {
		r := streamRecord{Record: guard.Record{Ev: event, St: st}}
		r.Rid = rid

		return r
	}

	tests := []struct {
		name    string
		rec     streamRecord
		rid     int64
		confirm bool
		want    bool
	}{
		{name: "confirm consumed", rec: rec(3, guard.EventConfirmConsumed, ""), rid: 3, confirm: true, want: true},
		{name: "confirm ignored", rec: rec(3, guard.EventConfirmIgnored, ""), rid: 3, confirm: true, want: true},
		{name: "confirm refused by revert event", rec: rec(3, guard.EventRevertRequested, ""), rid: 3, confirm: true, want: false},
		{name: "revert requested", rec: rec(4, guard.EventRevertRequested, ""), rid: 4, confirm: false, want: true},
		{name: "revert started", rec: rec(4, guard.EventRevertStart, ""), rid: 4, confirm: false, want: true},
		{name: "late ack counts for confirm", rec: rec(5, guard.EventCommitStart, lateAckStatus), rid: 5, confirm: true, want: true},
		{name: "late ack counts for revert", rec: rec(5, guard.EventState, lateAckStatus), rid: 5, confirm: false, want: true},
		{name: "legacy late event acks", rec: rec(6, guard.EventLateRequestAck, ""), rid: 6, confirm: true, want: true},
		{name: "mismatched rid never acks", rec: rec(9, guard.EventConfirmConsumed, ""), rid: 3, confirm: true, want: false},
		{name: "missing rid never acks", rec: rec(0, guard.EventConfirmConsumed, ""), rid: 3, confirm: true, want: false},
		{name: "zero request rid never acks", rec: rec(0, guard.EventConfirmConsumed, ""), rid: 0, confirm: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isWireAck(tt.rec, tt.rid, tt.confirm))
		})
	}
}

// TestWireStream_RecordBudgetCarriesLargeTxn pins that a TXN-sized record
// (the ~4KiB budget) survives the chunked feed intact.
func TestWireStream_RecordBudgetCarriesLargeTxn(t *testing.T) {
	t.Parallel()

	stream := newWireStream("wire-key")

	padding := strings.Repeat("x", 3000)
	line := `@PG2 {"v":1,"k":"wire-key","w":"g","seq":1,"ts":101,"ev":"TXN","rs":"` + padding + `"}`

	// Feed in small chunks to force heavy reassembly across many reads.
	for _, chunk := range splitChunks(line+"\r\n", 64) {
		stream.feed([]byte(chunk))
	}

	records, _ := stream.drain(0)

	require.Len(t, records, 1)
	assert.Len(t, records[0].Rs, 3000)
}

// splitChunks cuts data into size-byte chunks (the last one carries the
// remainder).
func splitChunks(data string, size int) []string {
	var chunks []string

	for len(data) > size {
		chunks = append(chunks, data[:size])
		data = data[size:]
	}

	if len(data) > 0 {
		chunks = append(chunks, data)
	}

	if len(chunks) == 0 {
		chunks = append(chunks, "")
	}

	return chunks
}
