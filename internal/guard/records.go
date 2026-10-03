package guard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Sentinel record protocol (spec 7, v2): the log is the durable transaction
// record. The guardian (and post-mortem converge) append one sentinel record per
// line into the free-form log; every line that is not a sentinel record for the
// current deploy key is display noise.
//
// Wire format: the "@PG2 " sentinel followed by one compact JSON object.
// encoding/json escapes control characters, so a record is always exactly one
// line no matter what the payload carries. The sentinel is a version carrier
// and an O(1) line discriminator, NOT anti-forgery: the per-deploy key inside
// the record remains the gate (nonce gating, spec 7). A line that fails to
// parse is noise, never a transaction event; a torn trailing write is skipped
// by the tail scanners (same tolerance as v1.3).

const (
	// RecordPrefix opens every record line and discriminates the protocol
	// version in O(1).
	RecordPrefix = "@PG2 "
	// RecordVersion is the envelope version this package reads and writes.
	RecordVersion = 1
	// WriterGuardian marks records written by the live guardian transaction.
	WriterGuardian = "g"
	// WriterDecide marks records written by the post-mortem converge.
	WriterDecide = "d"

	// RecordMaxLineBytes is the record budget (spec 7): STATE snapshots are
	// enforced to it at marshal time by dropping optional fields in a fixed
	// order (see FormatRecord); every other event relies on the excerpt bound
	// and stays advisory, and the parser accepts longer lines either way.
	RecordMaxLineBytes = 512
	// TxnMaxLineBytes bounds the TXN record (spec 7): the guardian checks the
	// marshal-time size against it and aborts with a minimal
	// FAILED_PRECONDITION when the embedded step lists cannot fit.
	TxnMaxLineBytes = 4096
	// ErrExcerptMaxBytes bounds the err excerpt at marshal time (raw bytes,
	// before JSON escaping).
	ErrExcerptMaxBytes = 200
	// ErrTruncatedSuffix marks a budget-truncated err excerpt.
	ErrTruncatedSuffix = "~"
	// LogTailScanBytes is the log tail the scanners read (spec 9.3: the tail
	// carries current state plus OLD/NEW/gen).
	LogTailScanBytes = 64 << 10
)

// Protocol events (spec 7): the v1.3 vocabulary, unchanged.
const (
	EventHello              = "HELLO"
	EventState              = "STATE"
	EventActivated          = "ACTIVATED"
	EventConfirmConsumed    = "CONFIRM_CONSUMED"
	EventConfirmIgnored     = "CONFIRM_IGNORED"
	EventRevertRequested    = "REVERT_REQUESTED"
	EventRevertStart        = "REVERT_START"
	EventReverted           = "REVERTED"
	EventRevertFailed       = "REVERT_FAILED"
	EventCommitStart        = "COMMIT_START"
	EventCommitted          = "COMMITTED"
	EventFailedPrecondition = "FAILED_PRECONDITION"
	EventOrphanKilled       = "ORPHAN_KILLED"
	EventLateRequestAck     = "LATE_REQUEST_ACK"
	EventTxn                = "TXN"
	EventLinkDown           = "LINK_DOWN"
	EventLinkDegraded       = "LINK_DEGRADED"
	EventExit               = "EXIT"
)

// Record is one protocol record. The envelope fields (V, K, W, Seq, TS, Ev)
// are required; the rest mirror the v1.3 marker fields per event and are
// optional (omitted when zero, so a missing numeric field reads as zero).
type Record struct {
	V   int    `json:"v"`
	K   string `json:"k"`
	W   string `json:"w"`
	Seq uint64 `json:"seq"`
	TS  int64  `json:"ts"`
	Ev  string `json:"ev"`

	St   string `json:"st,omitempty"`  // transaction status
	PID  int    `json:"pid,omitempty"` // guardian pid
	CPID int    `json:"cpid,omitempty"`
	Old  string `json:"old,omitempty"`
	New  string `json:"new,omitempty"`
	Gen  int64  `json:"gen,omitempty"`
	Mode string `json:"mode,omitempty"`
	Tier string `json:"tier,omitempty"`
	Pf   string `json:"pf,omitempty"` // profile path (HELLO)
	DL   int64  `json:"dl,omitempty"` // deadline, unix seconds (HELLO)
	RC   int    `json:"rc,omitempty"`
	Rs   string `json:"rs,omitempty"` // human reason (REVERT_START, CONFIRM_IGNORED)
	Err  string `json:"err,omitempty"`

	Rid int64  `json:"rid,omitempty"` // wire request id: acks for a wire command (spec 8)
	AA  string `json:"aa,omitempty"`  // TXN: embedded activation argv (JSON array)
	CA  string `json:"ca,omitempty"`  // TXN: embedded commit argv (JSON array)
	RA  string `json:"ra,omitempty"`  // TXN: embedded revert argv (JSON array)
	IV  string `json:"iv,omitempty"`  // TXN: embedded invariant target
}

// FormatRecord renders one record line: the sentinel plus the compact JSON
// object. Field order follows the struct declaration, so output is
// deterministic. The err excerpt is bounded at marshal time; STATE snapshots
// are additionally enforced to RecordMaxLineBytes by dropping optional fields
// in a fixed deterministic order (cosmetic first: tier, mode, gen, then dl,
// pf, rs, cpid, old, new, pid), truncating err last with a visible suffix and
// never dropping it entirely. Required envelope fields and the status are
// never dropped: a record whose envelope alone exceeds the budget ships over
// budget (correctness over the cap).
func FormatRecord(r Record) string {
	r.V = RecordVersion
	r.Err = BoundExcerpt(r.Err)

	if r.Ev == EventState {
		r = enforceRecordBudget(r)
	}

	return RecordPrefix + mustRecordJSON(r)
}

// recordOverBudget reports whether the marshaled record exceeds the budget.
func recordOverBudget(r Record) bool {
	return len(RecordPrefix)+len(mustRecordJSON(r)) > RecordMaxLineBytes
}

// dropNextStateField clears the next optional field in the deterministic drop
// order; false when every droppable field is already clear.
func dropNextStateField(r *Record) bool {
	switch {
	case r.Tier != "":
		r.Tier = ""
	case r.Mode != "":
		r.Mode = ""
	case r.Gen != 0:
		r.Gen = 0
	case r.DL != 0:
		r.DL = 0
	case r.Pf != "":
		r.Pf = ""
	case r.Rs != "":
		r.Rs = ""
	case r.CPID != 0:
		r.CPID = 0
	case r.Old != "":
		r.Old = ""
	case r.New != "":
		r.New = ""
	case r.PID != 0:
		r.PID = 0
	default:
		return false
	}

	return true
}

// enforceRecordBudget squeezes a STATE snapshot under the record budget: drop
// optional fields in the documented order first, then shrink the err excerpt
// (rune-safe halving) as the last resort, always leaving the visible
// truncation suffix once truncation happened.
func enforceRecordBudget(r Record) Record {
	for recordOverBudget(r) && dropNextStateField(&r) {
	}

	if !recordOverBudget(r) || r.Err == "" {
		return r
	}

	excerpt := r.Err
	for {
		r.Err = truncateRunes(excerpt, max(1, len(excerpt)/2)) + ErrTruncatedSuffix
		if !recordOverBudget(r) || excerpt == "" {
			return r
		}

		excerpt = r.Err[:len(r.Err)-len(ErrTruncatedSuffix)]
	}
}

// BoundExcerpt bounds an error excerpt for a record payload (spec 7):
// carriage returns are dropped (v1.3 single-line hygiene) and the raw form is
// cut at ErrExcerptMaxBytes without splitting a UTF-8 rune. JSON escaping at
// marshal time keeps the record single-line.
func BoundExcerpt(s string) string {
	return truncateRunes(strings.ReplaceAll(s, "\r", ""), ErrExcerptMaxBytes)
}

// truncateRunes cuts s to at most n bytes without splitting a UTF-8 rune.
func truncateRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}

	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}

	return s[:n]
}

// mustRecordJSON serializes a record; the struct carries only marshallable
// kinds, so a failure is unreachable and renders as an empty object.
func mustRecordJSON(r Record) string {
	b, err := json.Marshal(r)
	if err != nil {
		return "{}"
	}

	return string(b)
}

// RecordLineBytes returns the marshal-time size of one record line with the
// envelope version forced (spec 7): the TXN budget check the guardian runs
// before emitting, so an over-budget transaction aborts with a minimal
// FAILED_PRECONDITION instead of shipping a record post-mortem consumers
// cannot use.
func RecordLineBytes(r Record) int {
	r.V = RecordVersion

	return len(RecordPrefix) + len(mustRecordJSON(r))
}

// ParseRecordLine parses one log line into a Record. Lines without the
// sentinel, with an invalid envelope, or with a mismatched deploy key are
// display noise and rejected (nonce gating, spec 7); key == "" accepts any
// key (key recovery). Unknown JSON fields are ignored.
func ParseRecordLine(line, key string) (Record, bool) {
	if !strings.HasPrefix(line, RecordPrefix) {
		return Record{}, false
	}

	var r Record

	err := json.Unmarshal([]byte(line[len(RecordPrefix):]), &r)
	if err != nil {
		return Record{}, false
	}

	if r.V != RecordVersion || r.K == "" || r.Seq == 0 || r.TS <= 0 || r.Ev == "" {
		return Record{}, false
	}

	if r.W != WriterGuardian && r.W != WriterDecide {
		return Record{}, false
	}

	if key != "" && r.K != key {
		return Record{}, false
	}

	return r, true
}

// ParseRecords parses every valid record in a multi-line log excerpt (a
// remote tail view or a full read). Noise lines and, when key is non-empty,
// foreign keys are skipped.
func ParseRecords(content, key string) []Record {
	var out []Record

	for line := range strings.SplitSeq(content, "\n") {
		if r, ok := ParseRecordLine(line, key); ok {
			out = append(out, r)
		}
	}

	return out
}

// Folded is the folded state of one deploy key's records (the deployer's
// ScanContext surface): checkpoint fields take the newest value seen, terminal
// events latch the terminal flag with their exit code, and Last carries the
// newest record for event dispatch and sequence inheritance. This fold is the
// state inspect and converge will print (spec 9.3, stage 3).
type Folded struct {
	Found       bool
	Key         string
	Last        Record
	Status      string
	PID         int
	ChildPID    int
	Old         string
	New         string
	Gen         int64
	ProfilePath string
	Err         string
	RC          int
	Terminal    bool
	Mode        string
	Tier        string

	// TXN-embedded step lists (spec 7): the newest TXN record in the fold
	// wins. Nil when the tail carries no usable TXN record; consumers then
	// fall back to their own composition (never recompose silently).
	TxnActivation [][]string
	TxnCommit     [][]string
	TxnRevert     [][]string
	TxnInvariant  string
}

// IsTerminalEvent reports whether the event ends the transaction.
func IsTerminalEvent(ev string) bool {
	switch ev {
	case EventCommitted, EventReverted, EventRevertFailed, EventFailedPrecondition, EventExit:
		return true
	default:
		return false
	}
}

// Fold reduces parsed records to one transaction's state (spec 9.3). The fold
// covers the last record's key only, so mixed-key input folds the newest
// writer. Checkpoint fields (status, pid, child pid, old, new, gen, profile,
// err) take the newest non-empty value; terminal events latch Terminal with
// their rc; unknown events are ignored.
func Fold(records []Record) Folded {
	var f Folded
	if len(records) == 0 {
		return f
	}

	key := records[len(records)-1].K

	for _, r := range records {
		if r.K != key {
			continue
		}

		f.Found = true
		f.Key = key
		f.Last = r

		if r.St != "" {
			f.Status = r.St
		}

		if r.PID > 0 {
			f.PID = r.PID
		}

		if r.CPID > 0 {
			f.ChildPID = r.CPID
		}

		if r.Old != "" {
			f.Old = r.Old
		}

		if r.New != "" {
			f.New = r.New
		}

		if r.Gen > 0 {
			f.Gen = r.Gen
		}

		if r.Pf != "" {
			f.ProfilePath = r.Pf
		}

		if r.Mode != "" {
			f.Mode = r.Mode
		}

		if r.Tier != "" {
			f.Tier = r.Tier
		}

		if r.Err != "" {
			f.Err = r.Err
		}

		if r.Ev == EventTxn {
			f.TxnActivation = parseArgvJSON(r.AA)
			f.TxnCommit = parseArgvJSON(r.CA)
			f.TxnRevert = parseArgvJSON(r.RA)
			f.TxnInvariant = r.IV
		}

		if IsTerminalEvent(r.Ev) {
			f.Terminal = true
			f.RC = r.RC
		}
	}

	return f
}

// parseArgvJSON decodes a JSON array of argv arrays (the TXN embedding).
// Any parse failure yields nil: a torn or hostile TXN embedding must never
// fabricate step lists, and the consumer falls back to its own composition.
func parseArgvJSON(s string) [][]string {
	if s == "" {
		return nil
	}

	var steps [][]string

	err := json.Unmarshal([]byte(s), &steps)
	if err != nil {
		return nil
	}

	return steps
}

// ScanFile scans the log's tail (LogTailScanBytes) and folds the records of
// key, or of the last writer's key when key is empty (key recovery, replacing
// v1.3's candidate probing). A missing or empty log folds to the zero state; a
// torn trailing partial line and a line too large for the scan window are
// noise (spec 7).
func ScanFile(path, key string) (Folded, error) {
	records, err := scanTailRecords(path)
	if err != nil {
		return Folded{}, err
	}

	if key == "" {
		if len(records) == 0 {
			return Folded{}, nil
		}

		key = records[len(records)-1].K
	}

	filtered := make([]Record, 0, len(records))
	for _, r := range records {
		if r.K == key {
			filtered = append(filtered, r)
		}
	}

	return Fold(filtered), nil
}

// scanTailRecords reads the last LogTailScanBytes of the log, drops the torn
// trailing partial line, and parses every complete record (any key). A missing
// file is not an error.
func scanTailRecords(path string) ([]Record, error) {
	f, err := os.Open(path) //nolint:gosec // the log path is the caller-chosen slot's log by contract
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("open log for scan: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat log for scan: %w", err)
	}

	size := st.Size()
	if size == 0 {
		return nil, nil
	}

	start := max(size-LogTailScanBytes, 0)

	buf := make([]byte, size-start)

	n, err := f.ReadAt(buf, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read log tail: %w", err)
	}

	buf = buf[:n]

	// Drop a torn final write: without a trailing newline the last segment is
	// partial (spec 7). A line larger than the scan window has no newline in
	// it either and is dropped with it.
	if idx := bytes.LastIndexByte(buf, '\n'); idx >= 0 {
		buf = buf[:idx+1]
	} else {
		return nil, nil
	}

	var out []Record

	for line := range strings.SplitSeq(strings.TrimSuffix(string(buf), "\n"), "\n") {
		if r, ok := ParseRecordLine(strings.TrimSuffix(line, "\r"), ""); ok {
			out = append(out, r)
		}
	}

	return out, nil
}
