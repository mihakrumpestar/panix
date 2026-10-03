package guard

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// Deployer-side wire protocol (spec 8, 9.1): the spawn exec's output tap
// (Executioner WithOutputTap) delivers the relay's raw PTY chunks, and this
// file turns them into records for the confirm loop while the narrative keeps
// flowing through the normal CommandLog path untouched. The frames panix
// writes back ride the stdin pipe (Executioner WithStdin).

// Command-frame verbs and the ack vocabulary (the guardian-side contract):
// panix writes {"c":"confirm","rid":N} / {"c":"revert","rid":N}, one line
// each, and the guardian answers with log records carrying the matching rid.
const (
	frameConfirm = "confirm"
	frameRevert  = "revert"

	// lateAckStatus marks the ack a transaction emits after COMMIT_START
	// (spec 8: the status-late ack; the request may already have taken
	// effect). A matching-rid record with this status is an ack regardless
	// of its event.
	lateAckStatus = "late"
)

// commandFrame is the shape of the frames panix writes to the guardian's
// command pipe. The same shape doubles as the echo-defense predicate: an
// inbound line that parses as a command frame is panix's own frame echoed
// back by the PTY line discipline, never an event, and is dropped before
// record parsing (belt and braces on top of the executioner's echo disable).
type commandFrame struct {
	C   string `json:"c"`
	Rid int64  `json:"rid"`
}

// formatFrame renders one command frame line (newline-terminated). The
// marshal error is unreachable: the struct carries only marshallable kinds,
// and the empty render keeps the caller's best-effort write honest.
func formatFrame(command string, rid int64) string {
	b, err := json.Marshal(commandFrame{C: command, Rid: rid})
	if err != nil {
		return ""
	}

	return string(b) + "\n"
}

// isCommandFrame reports whether a raw line is a command frame shape. The
// sentinel-guarded record lines can never match, so the predicate is safe to
// run ahead of record parsing.
func isCommandFrame(line []byte) bool {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}

	var frame commandFrame

	return json.Unmarshal(trimmed, &frame) == nil && frame.C != ""
}

// ridProbe extracts the wire correlation field from an already-validated
// record line. internal/guard owns the envelope schema and ignores unknown
// fields, so the rid rides beside it until the record struct grows the field.
type ridProbe struct {
	Rid int64 `json:"rid"`
}

// parseWireRecord validates one record line for the deploy key (nonce
// gating, spec 7) and extracts its rid; 0 means the record carries no rid.
func parseWireRecord(line, key string) (streamRecord, bool) {
	record, ok := guard.ParseRecordLine(line, key)
	if !ok {
		return streamRecord{}, false
	}

	var probe ridProbe

	_ = json.Unmarshal([]byte(line[len(guard.RecordPrefix):]), &probe)

	return streamRecord{Record: record, Rid: probe.Rid}, true
}

// streamRecord is one record observed on the live wire with its correlation
// id (0 when the record carries none).
type streamRecord struct {
	guard.Record

	Rid int64
}

// isWireAck reports whether a record acks the request with the given rid
// (the guardian-side contract: records without rid or with a mismatched rid
// are never acks). Confirm waits for CONFIRM_CONSUMED/CONFIRM_IGNORED,
// revert for REVERT_REQUESTED/REVERT_START, and the status-late ack counts
// for either.
func isWireAck(rec streamRecord, rid int64, confirm bool) bool {
	if rid == 0 || rec.Rid != rid {
		return false
	}

	if rec.St == lateAckStatus || rec.Ev == guard.EventLateRequestAck {
		return true
	}

	if confirm {
		return rec.Ev == guard.EventConfirmConsumed || rec.Ev == guard.EventConfirmIgnored
	}

	return rec.Ev == guard.EventRevertRequested || rec.Ev == guard.EventRevertStart
}

// streamRecordCap bounds the wire record buffer. The consumer falls behind
// only during the remote health checks; beyond the cap the oldest records
// compact away and the drain cursor shifts with them.
const streamRecordCap = 4096

// pendingLineCap bounds the torn-line accumulator: a stream that never
// produces a newline is narrative noise (records are bounded lines), so the
// accumulator resets instead of growing without bound.
const pendingLineCap = 1 << 20

// wireStream consumes the spawn exec's output tap: it reassembles lines from
// raw PTY chunks, drops echoed command frames, parses the @PG2 records for
// the deploy key, and exposes them to the confirm loop. feed runs on the
// exec's read goroutine and never blocks on the consumer: the mutex is held
// only for bounded parse work and the loop's drain copy.
type wireStream struct {
	key     string
	mu      sync.Mutex
	pending []byte
	base    int            // ordinal of records[0] (shifts on compaction)
	records []streamRecord // appended in arrival order
	notify  chan struct{}  // cap 1: a level-ish wakeup token
}

// newWireStream builds a stream gating records on the per-deploy key.
func newWireStream(key string) *wireStream {
	return &wireStream{key: key, notify: make(chan struct{}, 1)}
}

// feed implements the WithOutputTap contract: ordered raw chunks, must not
// block, chunk ownership transfers in. A notify token is left behind for the
// consumer; a collapsed token is harmless because the drain always returns
// every record since the consumer's cursor.
func (s *wireStream) feed(chunk []byte) {
	s.mu.Lock()
	s.pending = append(s.pending, chunk...)

	for {
		idx := bytes.IndexByte(s.pending, '\n')
		if idx < 0 {
			break
		}

		line := s.pending[:idx]
		s.pending = s.pending[idx+1:]

		s.ingestLine(line)
	}

	if len(s.pending) > pendingLineCap {
		// A newline-less stretch this large is noise (record lines are
		// bounded); drop the torn line and keep the stream healthy.
		s.pending = nil
	}
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// ingestLine classifies one complete line (mutex held): echoed command
// frames are dropped, narrative stays with the CommandLog path, and valid
// records for the deploy key are appended.
func (s *wireStream) ingestLine(line []byte) {
	trimmed := bytes.TrimSuffix(line, []byte("\r")) // PTY ONLCR surfaces \n as \r\n

	if isCommandFrame(trimmed) {
		return
	}

	if !bytes.HasPrefix(trimmed, []byte(guard.RecordPrefix)) {
		return
	}

	if rec, ok := parseWireRecord(string(trimmed), s.key); ok {
		s.records = append(s.records, rec)

		if drop := len(s.records) - streamRecordCap; drop > 0 {
			s.records = append(s.records[:0], s.records[drop:]...)
			s.base += drop
		}
	}
}

// ingestTail feeds records parsed from a log-tail read (the confirm loop's
// floor, spec 9.3) into the same record path, so both transports share the
// cursor and the fold. Re-delivered records are idempotent for the loop's
// decisions (latched flags, rid-matched acks, terminal latch).
func (s *wireStream) ingestTail(tail string) {
	s.mu.Lock()

	for _, record := range guard.ParseRecords(tail, s.key) {
		s.records = append(s.records, streamRecord{Record: record})

		if drop := len(s.records) - streamRecordCap; drop > 0 {
			s.records = append(s.records[:0], s.records[drop:]...)
			s.base += drop
		}
	}
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// drain returns every record appended after the given ordinal and the new
// high-water mark. A cursor older than the buffer's start re-drives from the
// oldest retained record (drop-oldest compaction).
func (s *wireStream) drain(after int) ([]streamRecord, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := min(max(after-s.base, 0), len(s.records))

	out := make([]streamRecord, len(s.records)-start)
	copy(out, s.records[start:])

	return out, s.base + len(s.records)
}

// snapshot folds the wire records into the deployer's transaction context
// (spec 7: checkpoint fields take the newest non-empty value, terminal
// events latch).
func (s *wireStream) snapshot() guard.Folded {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]guard.Record, len(s.records))
	for i := range s.records {
		records[i] = s.records[i].Record
	}

	return guard.Fold(records)
}
