package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rec renders a fixture record line with the envelope filled in.
func rec(key, writer string, seq uint64, ts int64, ev string, r Record) string {
	r.K, r.W, r.Seq, r.TS, r.Ev = key, writer, seq, ts, ev

	return FormatRecord(r)
}

// TestRecordGolden pins the wire format: sentinel plus compact JSON with the
// struct's deterministic field order.
func TestRecordGolden(t *testing.T) {
	got := rec("k", WriterGuardian, 1, 5, EventState, Record{St: "activated"})

	want := `@PG2 {"v":1,"k":"k","w":"g","seq":1,"ts":5,"ev":"STATE","st":"activated"}`
	if got != want {
		t.Fatalf("golden record: got %q, want %q", got, want)
	}

	got = rec("k", WriterDecide, 2, 6, EventRevertFailed, Record{St: "revert_failed", RC: 1})

	want = `@PG2 {"v":1,"k":"k","w":"d","seq":2,"ts":6,"ev":"REVERT_FAILED","st":"revert_failed","rc":1}`
	if got != want {
		t.Fatalf("writer tag must be pinned: got %q, want %q", got, want)
	}
}

// TestRecordRoundTrip marshals and parses adversarial payloads: spaces,
// equals, backslashes, quotes, control characters and unicode must survive the
// JSON round-trip and keep the record on one line.
func TestRecordRoundTrip(t *testing.T) {
	cases := []Record{
		{St: "activated", PID: 42, CPID: 7, Old: "/nix/store/old", New: "/nix/store/new", Gen: 5, Mode: "switch", Tier: "full", Pf: "/nix/var/nix/profiles/system", DL: 99, RC: 0, Rs: "reason", Err: "boom"},
		{Old: `path with spaces and = signs and \ backslashes`},
		{New: `quote "double" and 'single' and tab\tliteral`},
		{Err: "line one\nline two\rline three\x00nul\x1b[31mcolor"},
		{Old: "unicode: héllo — 世界 🚀", New: "emoji family 👨‍👩‍👧‍👦"},
		{St: "state\x01with\x02control\x7fbytes"},
		{K: `key with "quote" and \ backslash`},
		{Gen: -1, DL: -1, RC: -3, PID: -1},
	}

	for i, want := range cases {
		key := want.K
		if key == "" {
			key = "deploy-key"
		}

		line := rec(key, WriterGuardian, uint64(i+1), 1000, EventState, want)
		if strings.ContainsAny(line, "\n\r") {
			t.Fatalf("case %d: record must stay on one line: %q", i, line)
		}

		got, ok := ParseRecordLine(line, key)
		if !ok {
			t.Fatalf("case %d: parse failed for %q", i, line)
		}

		want.V = RecordVersion
		want.K, want.W, want.Seq, want.TS, want.Ev = key, WriterGuardian, uint64(i+1), 1000, EventState
		want.Err = BoundExcerpt(want.Err)

		if got != want {
			t.Fatalf("case %d: round-trip mismatch:\n got %+v\nwant %+v", i, got, want)
		}
	}
}

// TestRecordParseRejects pins the noise rules: wrong sentinel, broken JSON,
// invalid envelope fields and foreign keys are all display noise.
func TestRecordParseRejects(t *testing.T) {
	cases := []string{
		"",
		"some child output line",
		"@PG1 k 1 5 STATE status=activating", // legacy protocol prefix
		"@PG2 not json at all",
		`@PG2 ["an","array"]`,
		`@PG2 {"v":1,"k":"k","w":"g","seq":1,"ts":5,"ev":"STATE"} trailing`,
		`@PG2 {"v":2,"k":"k","w":"g","seq":1,"ts":5,"ev":"STATE"}`,     // wrong version
		`@PG2 {"v":1,"w":"g","seq":1,"ts":5,"ev":"STATE"}`,             // missing key
		`@PG2 {"v":1,"k":"","w":"g","seq":1,"ts":5,"ev":"STATE"}`,      // empty key
		`@PG2 {"v":1,"k":"k","w":"x","seq":1,"ts":5,"ev":"STATE"}`,     // unknown writer
		`@PG2 {"v":1,"k":"k","seq":1,"ts":5,"ev":"STATE"}`,             // missing writer
		`@PG2 {"v":1,"k":"k","w":"g","ts":5,"ev":"STATE"}`,             // missing seq
		`@PG2 {"v":1,"k":"k","w":"g","seq":0,"ts":5,"ev":"STATE"}`,     // zero seq
		`@PG2 {"v":1,"k":"k","w":"g","seq":1,"ev":"STATE"}`,            // missing ts
		`@PG2 {"v":1,"k":"k","w":"g","seq":1,"ts":0,"ev":"STATE"}`,     // zero ts
		`@PG2 {"v":1,"k":"k","w":"g","seq":1,"ts":5}`,                  // missing event
		`@PG2 {"v":1,"k":"k","w":"g","seq":1,"ts":5,"ev":""}`,          // empty event
		`@PG2 {"v":1,"k":"other","w":"g","seq":1,"ts":5,"ev":"STATE"}`, // foreign key
	}

	for _, line := range cases {
		if _, ok := ParseRecordLine(line, "k"); ok {
			t.Errorf("expected rejection for %q", line)
		}
	}

	// Key recovery mode accepts the foreign key.
	if _, ok := ParseRecordLine(`@PG2 {"v":1,"k":"other","w":"g","seq":1,"ts":5,"ev":"STATE"}`, ""); !ok {
		t.Error("empty key must accept any deploy key")
	}
}

// TestBoundExcerpt pins the excerpt bound: carriage returns dropped, raw bytes
// bounded, no rune split at the cut.
func TestBoundExcerpt(t *testing.T) {
	if got := BoundExcerpt("a\rb\rc"); got != "abc" {
		t.Fatalf("carriage returns must be dropped, got %q", got)
	}

	if got := BoundExcerpt(strings.Repeat("a", 500)); len(got) != ErrExcerptMaxBytes {
		t.Fatalf("excerpt not bounded: %d bytes", len(got))
	}

	// The cut must not split a rune: fill to the boundary with ASCII and put a
	// multi-byte rune across it.
	prefix := strings.Repeat("a", ErrExcerptMaxBytes-1)

	got := BoundExcerpt(prefix + "é")
	if got != prefix {
		t.Fatalf("cut split a rune: %q", got[len(got)-4:])
	}
}

// TestFormatRecordStateBudget pins the STATE budget enforcement: the line fits
// the budget, fields drop in the documented order, and err truncates last with
// the visible suffix and is never dropped entirely.
func TestFormatRecordStateBudget(t *testing.T) {
	big := strings.Repeat("p", 150)

	r := Record{
		K: "k", W: WriterGuardian, Seq: 1, TS: 5, Ev: EventState,
		St:   "activating",
		PID:  42,
		CPID: 7,
		Old:  "/nix/store/" + big,
		New:  "/nix/store/" + big,
		Gen:  9,
		Mode: "switch",
		Tier: "full",
		Pf:   "/nix/var/nix/profiles/" + big,
	}

	line := FormatRecord(r)
	if len(line) > RecordMaxLineBytes {
		t.Fatalf("STATE line over budget: %d bytes: %q", len(line), line)
	}

	parsed, ok := ParseRecordLine(line, "k")
	if !ok {
		t.Fatalf("budget record must still parse: %q", line)
	}

	// Cosmetic fields drop first, the functional tail survives.
	if parsed.Tier != "" || parsed.Mode != "" || parsed.Gen != 0 || parsed.Pf != "" {
		t.Fatalf("cosmetic fields must drop first: %+v", parsed)
	}

	if parsed.St != "activating" || parsed.PID != 42 || parsed.CPID != 7 || parsed.Old == "" || parsed.New == "" {
		t.Fatalf("functional fields must survive: %+v", parsed)
	}

	// Deep overflow: the deterministic order drops old before new and the pid
	// last of the droppable fields.
	huge := strings.Repeat("p", 400)
	line = FormatRecord(Record{
		K: "k", W: WriterGuardian, Seq: 1, TS: 5, Ev: EventState,
		St: "activating", PID: 42, CPID: 7,
		Old: "/nix/store/" + huge, New: "/nix/store/" + huge,
	})

	parsed, ok = ParseRecordLine(line, "k")
	if !ok || len(line) > RecordMaxLineBytes {
		t.Fatalf("deep-overflow record must parse under budget: ok=%v len=%d", ok, len(line))
	}

	if parsed.Old != "" || parsed.New == "" || parsed.PID != 42 || parsed.CPID != 0 {
		t.Fatalf("drop order violated: %+v", parsed)
	}

	// The heartbeat status must survive every drop round.
	r.St = "committing"
	line = FormatRecord(r)

	parsed, ok = ParseRecordLine(line, "k")
	if !ok || parsed.St != "committing" {
		t.Fatalf("status must never drop: %q", line)
	}

	// Err survives whole while the drop order still makes room (truncation
	// is the last resort, after every droppable field is gone).
	r.Err = strings.Repeat("e", 400)
	line = FormatRecord(r)

	parsed, ok = ParseRecordLine(line, "k")
	if !ok {
		t.Fatalf("record with err must parse: %q", line)
	}

	if len(line) > RecordMaxLineBytes {
		t.Fatalf("STATE line with err over budget: %d bytes", len(line))
	}

	if parsed.Err != BoundExcerpt(r.Err) || strings.HasSuffix(parsed.Err, ErrTruncatedSuffix) {
		t.Fatalf("err must survive untruncated while drops make room: %q", parsed.Err)
	}

	// With the drop order exhausted (a pathological envelope), the excerpt
	// shrinks to fit and keeps the visible suffix; it is never dropped
	// entirely.
	pathological := Record{
		K: strings.Repeat("k", 400), W: WriterGuardian, Seq: 1, TS: 5, Ev: EventState,
		St: "activating", Err: strings.Repeat("e", 400),
	}
	line = FormatRecord(pathological)

	parsed, ok = ParseRecordLine(line, pathological.K)
	if !ok {
		t.Fatalf("pathological record must parse: %q", line)
	}

	if parsed.Err == "" || !strings.HasSuffix(parsed.Err, ErrTruncatedSuffix) {
		t.Fatalf("truncated err must keep the visible suffix: %q", parsed.Err)
	}
}

// TestScanTail pins the torn-tail tolerance: partial last lines, oversized
// lines and foreign-format lines are noise; the last valid record wins.
func TestScanTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")

	content := strings.Join([]string{
		"some child output",
		rec("k", WriterGuardian, 1, 100, EventState, Record{St: "activating"}),
		"another child line",
		rec("other", WriterGuardian, 9, 200, EventState, Record{St: "x"}),
		rec("k", WriterGuardian, 2, 300, EventActivated, Record{St: "activated", PID: 4242}),
		`@PG2 {"v":1,"k":"k","w":"g","seq":3,"ts":4` + strings.Repeat(" noise", 20), // malformed JSON line
		"@PG1 k 4 5 STATE status=activating",                                        // legacy protocol line
		"partial torn line without newline",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	records, err := scanTailRecords(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 valid records (both keys), got %d: %+v", len(records), records)
	}

	if records[len(records)-1].Seq != 2 || records[len(records)-1].Ev != EventActivated {
		t.Fatalf("wrong last record: %+v", records[len(records)-1])
	}

	// The oversized line is dropped with the torn tail: a single line larger
	// than the scan window leaves no complete record.
	huge := strings.Repeat("x", LogTailScanBytes+1024) + "\n" + "trailing noise without newline"
	if err := os.WriteFile(path, []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}

	records, err = scanTailRecords(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 0 {
		t.Fatalf("oversized line must yield no records, got %d", len(records))
	}

	// A valid record after the oversized line is still found (the fixture
	// ends with a newline so the record is not a torn tail).
	if err := os.WriteFile(path, []byte(huge+"\n"+rec("k", WriterGuardian, 5, 9, EventCommitted, Record{St: "committed"})+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := ScanFile(path, "k")
	if err != nil || !f.Found || f.Last.Ev != EventCommitted {
		t.Fatalf("record after oversized line must be found: found=%v err=%v", f.Found, err)
	}
}

// TestScanFilePins the file-scan surface: missing and empty logs fold to the
// zero state, the fold covers the requested key, and empty-key scans recover
// the last writer's key.
func TestScanFile(t *testing.T) {
	dir := t.TempDir()

	if f, err := ScanFile(filepath.Join(dir, "log"), "k"); err != nil || f.Found {
		t.Fatalf("missing log: found=%v err=%v", f.Found, err)
	}

	path := filepath.Join(dir, "log")

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if f, err := ScanFile(path, "k"); err != nil || f.Found {
		t.Fatalf("empty log: found=%v err=%v", f.Found, err)
	}

	content := strings.Join([]string{
		rec("old-tx", WriterGuardian, 1, 10, EventState, Record{St: "activating", PID: 1}),
		rec("cur-tx", WriterGuardian, 1, 20, EventHello, Record{PID: 2, Old: "/nix/store/o", New: "/nix/store/n", Gen: 3}),
		rec("cur-tx", WriterGuardian, 2, 30, EventState, Record{St: "activating", PID: 2, CPID: 77}),
		"partial torn line",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := ScanFile(path, "cur-tx")
	if err != nil {
		t.Fatal(err)
	}

	if !f.Found || f.Status != "activating" || f.PID != 2 || f.ChildPID != 77 || f.Gen != 3 || f.Old != "/nix/store/o" || f.New != "/nix/store/n" {
		t.Fatalf("wrong fold for the requested key: %+v", f)
	}

	if f.Last.Seq != 2 {
		t.Fatalf("last record must carry the sequence for inheritance: %+v", f.Last)
	}

	// Key recovery: the last writer's key, folding only that writer's records.
	recovered, err := ScanFile(path, "")
	if err != nil {
		t.Fatal(err)
	}

	if recovered.Key != "cur-tx" || recovered.PID != 2 {
		t.Fatalf("key recovery must fold the last writer: %+v", recovered)
	}
}

// TestFold pins the fold table (the deployer's ScanContext semantics): STATE
// snapshots set the checkpoint fields, terminal events latch terminal+rc, and
// unknown events are ignored.
func TestFold(t *testing.T) {
	t.Run("live transaction stays non-terminal", func(t *testing.T) {
		f := Fold([]Record{
			{K: "k", Ev: EventHello, PID: 4242, Old: "o", New: "n", Gen: 7, Pf: "/nix/var/nix/profiles/system"},
			{K: "k", Ev: EventState, St: "activating", PID: 4242},
			{K: "k", Ev: EventActivated, St: "activated", PID: 4242},
		})

		if !f.Found || f.Terminal || f.Status != "activated" || f.PID != 4242 || f.Gen != 7 {
			t.Fatalf("wrong fold: %+v", f)
		}

		if f.ProfilePath != "/nix/var/nix/profiles/system" {
			t.Fatalf("the HELLO pf must reach the fold (the post-mortem profile restore): %+v", f)
		}
	})

	t.Run("an empty pf never fabricates a profile", func(t *testing.T) {
		f := Fold([]Record{{K: "k", Ev: EventState, St: "activating"}})

		if f.ProfilePath != "" {
			t.Fatalf("records without pf must leave the profile empty: %+v", f)
		}
	})

	t.Run("terminal events latch terminal and rc", func(t *testing.T) {
		for _, ev := range []string{EventCommitted, EventReverted, EventRevertFailed, EventFailedPrecondition, EventExit} {
			f := Fold([]Record{{K: "k", Ev: ev, St: "s", RC: 3}})

			if !f.Terminal || f.RC != 3 {
				t.Fatalf("event %s must be terminal with rc: %+v", ev, f)
			}
		}

		f := Fold([]Record{
			{K: "k", Ev: EventRevertStart, Rs: "activation failed", Err: "boom"},
			{K: "k", Ev: EventReverted, St: "reverted", RC: 2, Err: "boom detail"},
		})

		if !f.Terminal || f.RC != 2 || f.Err != "boom detail" {
			t.Fatalf("reverted fold: %+v", f)
		}
	})

	t.Run("a trailing STATE after a terminal event stays terminal", func(t *testing.T) {
		// The heartbeat can race the terminal records; the fold is monotonic so
		// the deployer never decides a committed slot.
		f := Fold([]Record{
			{K: "k", Ev: EventCommitted, St: "committed", RC: 0},
			{K: "k", Ev: EventState, St: "committed", PID: 4242},
		})

		if !f.Terminal {
			t.Fatalf("terminal must latch across a trailing snapshot: %+v", f)
		}
	})

	t.Run("unknown events are ignored", func(t *testing.T) {
		f := Fold([]Record{
			{K: "k", Ev: "SOMETHING_NEW", St: "mysterious"},
			{K: "k", Ev: EventState, St: "activating"},
		})

		if f.Status != "activating" || f.Terminal {
			t.Fatalf("unknown events must not disturb the fold: %+v", f)
		}
	})

	t.Run("mixed keys fold the newest writer", func(t *testing.T) {
		f := Fold([]Record{
			{K: "old", Ev: EventState, St: "activating", PID: 1},
			{K: "new", Ev: EventState, St: "activated", PID: 2},
		})

		if f.Key != "new" || f.PID != 2 || f.Status != "activated" {
			t.Fatalf("fold must cover the last record's key: %+v", f)
		}
	})

	t.Run("empty input folds to zero", func(t *testing.T) {
		if f := Fold(nil); f.Found || f.Terminal {
			t.Fatalf("empty fold must be zero: %+v", f)
		}
	})
}

// TestParseRecords pins the excerpt parsing over a multi-line log view.
func TestParseRecords(t *testing.T) {
	content := strings.Join([]string{
		rec("k", WriterGuardian, 1, 1, EventHello, Record{PID: 1}),
		"noise",
		rec("other", WriterGuardian, 2, 2, EventState, Record{St: "x"}),
	}, "\n")

	if got := ParseRecords(content, "k"); len(got) != 1 || got[0].PID != 1 {
		t.Fatalf("key filter must skip foreign records: %+v", got)
	}

	if got := ParseRecords(content, ""); len(got) != 2 {
		t.Fatalf("empty key must accept every record: %+v", got)
	}
}

// TestRecordRidRoundTrip pins the wire correlation field: rid rides the
// envelope, survives the round trip, and omits from records without one so
// the golden shape of rid-less records is unchanged.
func TestRecordRidRoundTrip(t *testing.T) {
	line := rec("k", WriterGuardian, 1, 5, EventConfirmConsumed, Record{Rid: 42})

	if !strings.Contains(line, `"rid":42`) {
		t.Fatalf("rid must marshal: %q", line)
	}

	got, ok := ParseRecordLine(line, "k")
	if !ok || got.Rid != 42 {
		t.Fatalf("rid round trip failed: ok=%v rid=%d", ok, got.Rid)
	}

	plain := rec("k", WriterGuardian, 2, 5, EventState, Record{St: "activating"})
	if strings.Contains(plain, "rid") {
		t.Fatalf("rid must omit when zero: %q", plain)
	}

	got, ok = ParseRecordLine(plain, "k")
	if !ok || got.Rid != 0 {
		t.Fatalf("rid-less record must parse with rid 0: ok=%v rid=%d", ok, got.Rid)
	}
}

// TestTxnRecordRoundTrip pins the TXN embedding: the step lists travel as
// JSON string arrays and fold back into decoded lists.
func TestTxnRecordRoundTrip(t *testing.T) {
	r := Record{
		AA: `[["/nix/store/new/bin/switch-to-configuration","test"]]`,
		CA: `[["nix-env","-p","/p","--set","/nix/store/new"]]`,
		RA: `[["/nix/store/old/bin/switch-to-configuration","switch"]]`,
		IV: "/nix/store/new",
	}

	line := rec("k", WriterGuardian, 1, 5, EventTxn, r)

	got, ok := ParseRecordLine(line, "k")
	if !ok {
		t.Fatalf("TXN record must parse: %q", line)
	}

	if got.AA != r.AA || got.CA != r.CA || got.RA != r.RA || got.IV != r.IV {
		t.Fatalf("TXN fields must round trip: %+v", got)
	}

	f := Fold([]Record{got})
	if len(f.TxnCommit) != 1 || f.TxnInvariant != "/nix/store/new" {
		t.Fatalf("fold must decode the TXN lists: %+v", f)
	}

	if len(f.TxnCommit[0]) != 5 || f.TxnCommit[0][3] != "--set" {
		t.Fatalf("decoded commit list wrong: %+v", f.TxnCommit)
	}

	// A hostile or torn embedding yields nil lists, never fabricated steps.
	bad := Record{AA: "not json", CA: `["flat"]`}
	line = rec("k", WriterGuardian, 2, 5, EventTxn, bad)

	got, ok = ParseRecordLine(line, "k")
	if !ok {
		t.Fatalf("malformed TXN payload must still parse as a record: %q", line)
	}

	f = Fold([]Record{got})
	if f.TxnActivation != nil || f.TxnCommit != nil {
		t.Fatalf("malformed embeddings must decode to nil: %+v", f)
	}
}

// TestTxnBudget pins the TXN size surface: small transactions fit the
// ~4KiB budget, a pathological payload exceeds it, and RecordLineBytes
// measures the real line the guardian would ship.
func TestTxnBudget(t *testing.T) {
	aa := `[["` + strings.Repeat("a", 200) + `"]]`

	small := Record{
		K: "k", W: WriterGuardian, Seq: 1, TS: 5, Ev: EventTxn,
		AA: aa, CA: `[["nix-env","--set","/nix/store/new"]]`, RA: `[["/nix/store/old/bin/activate"]]`, IV: "/nix/store/new",
	}
	if RecordLineBytes(small) > TxnMaxLineBytes {
		t.Fatalf("small TXN must fit the budget: %d", RecordLineBytes(small))
	}

	huge := Record{
		K: "k", W: WriterGuardian, Seq: 1, TS: 5, Ev: EventTxn,
		AA: `[[` + strings.Repeat(`"`+strings.Repeat("a", 3000)+`",`, 3) + `"x"]]`,
	}
	if RecordLineBytes(huge) <= TxnMaxLineBytes {
		t.Fatalf("pathological TXN must exceed the budget: %d", RecordLineBytes(huge))
	}

	// STATE records keep their own tighter budget surface untouched.
	st := Record{K: "k", W: WriterGuardian, Seq: 1, TS: 5, Ev: EventState, St: "activating"}
	if RecordLineBytes(st) > RecordMaxLineBytes {
		t.Fatalf("STATE budget must still hold: %d", RecordLineBytes(st))
	}
}

// TestLinkEvents pins the log-only link events: they parse and fold as
// non-terminal records.
func TestLinkEvents(t *testing.T) {
	for _, ev := range []string{EventLinkDown, EventLinkDegraded} {
		line := rec("k", WriterGuardian, 1, 5, ev, Record{Rs: "wire"})

		got, ok := ParseRecordLine(line, "k")
		if !ok || got.Ev != ev {
			t.Fatalf("event %s must parse: %q", ev, line)
		}

		f := Fold([]Record{got})
		if f.Terminal {
			t.Fatalf("event %s must not be terminal: %+v", ev, f)
		}
	}
}
