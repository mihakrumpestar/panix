package main

import (
	"encoding/json"
	"fmt"
	"github.com/mihakrumpestar/panix/internal/guard"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// wireHarness wires one transaction to a real duplex pipe pair exactly like
// guardianEntry does, so tests exercise the production wiring: records dual-
// write (log first, then the event pipe), commands arrive on the command
// pipe, and the link-health records stay log-only.
type wireHarness struct {
	txn  *transaction
	wire *wireWriter
	lock *slotLock
	logw *LogWriter

	evtR *os.File
	evtW *os.File
	cmdR *os.File
	cmdW *os.File

	outBuf   strings.Builder
	outMu    sync.Mutex
	outDone  chan struct{}
	dropHook func()
}

func newWireHarness(t *testing.T, cfg *startConfig) *wireHarness {
	t.Helper()

	lock, err := openLockedLog(cfg.dir)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}

	logw, err := OpenLogWriter(cfg.dir)
	if err != nil {
		t.Fatalf("log writer: %v", err)
	}

	evtR, evtW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	cmdR, cmdW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	h := &wireHarness{
		lock: lock, logw: logw,
		evtR: evtR, evtW: evtW, cmdR: cmdR, cmdW: cmdW,
		outDone: make(chan struct{}),
	}

	// Collect the wire output concurrently: the writer goroutine must never
	// block on the test's reading pace (spec 6.1).
	go func() {
		buf := make([]byte, 4096)

		for {
			n, err := evtR.Read(buf)
			if n > 0 {
				h.outMu.Lock()
				h.outBuf.Write(buf[:n])
				h.outMu.Unlock()
			}

			if err != nil {
				close(h.outDone)

				return
			}
		}
	}()

	h.wire = newWireWriter(evtW, nil, nil)
	h.txn = newTransaction(cfg, logw, lock.file, h.wire, cmdR)
	h.wire.setHooks(
		func() {
			h.txn.core.emitLogOnly(guard.EventLinkDegraded, guard.Record{Rs: "wire frame dropped (slow reader)"})

			if h.dropHook != nil {
				h.dropHook()
			}
		},
		func(err error) {
			h.txn.core.emitLogOnly(guard.EventLinkDown, guard.Record{Rs: wireDownReason(err)})
		},
	)

	t.Cleanup(func() { h.teardown() })

	return h
}

// run drives the transaction to its terminal state while feed runs alongside
// (command frames, timings), then tears the wire down and returns the
// terminal state, the full log, and the wire output.
func (h *wireHarness) run(feed func(w io.Writer)) (txState, string, string) {
	if feed != nil {
		go feed(h.cmdW)
	}

	st := h.txn.run()

	h.wire.close()
	_ = h.evtW.Close() // EOF for the collector
	<-h.outDone

	h.outMu.Lock()
	out := h.outBuf.String()
	h.outMu.Unlock()

	return st, h.readLog(), out
}

func (h *wireHarness) readLog() string {
	b, err := os.ReadFile(h.txn.cfg.dir + "/" + logName)
	if err != nil {
		return ""
	}

	return string(b)
}

func (h *wireHarness) teardown() {
	_ = h.lock.Close()
	_ = h.logw.Close()
	_ = h.evtR.Close()
	_ = h.cmdR.Close()
	_ = h.cmdW.Close()
}

// writeFrame writes one wire command frame (newline-terminated).
func writeFrame(w io.Writer, command string, rid int64) {
	_, _ = io.WriteString(w, formatTestFrame(command, rid))
}

// formatTestFrame renders one command frame line (mirrors the deployer's
// shape, which the guardian reads).
func formatTestFrame(command string, rid int64) string {
	b, err := json.Marshal(wireCommandFrame{C: command, Rid: rid})
	if err != nil {
		return ""
	}

	return string(b) + "\n"
}

// wireAutoCfg builds an auto-tier config whose activation succeeds and whose
// commit runs one stub step.
func wireAutoCfg(t *testing.T, slot, stubs string) *startConfig {
	t.Helper()

	cfg := newTestCfg(slot)
	cfg.oldClosure, cfg.newClosure, cfg.gen = "/nix/store/old", "/nix/store/new", 5
	cfg.profilePath = fakeProfile(t, stubs, 5, cfg.oldClosure)
	cfg.nixEnv = writeStub(t, stubs, "nix-env", nixEnvBody())
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", "exit 0\n")}}
	cfg.commitArgv = [][]string{{cfg.nixEnv, "-p", cfg.profilePath, "--set", cfg.newClosure}}
	cfg.revertArgv = [][]string{{writeStub(t, stubs, "revert", "exit 0\n")}}
	cfg.invariantTarget = cfg.newClosure

	return cfg
}

// TestWireDualWriteOrdering pins the dual-write contract: every record that
// reaches the wire first landed in the log, and the wire frames arrive in
// record order (single writer goroutine).
func TestWireDualWriteOrdering(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)

	h := newWireHarness(t, cfg)
	st, log, out := h.run(nil)

	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	wireLines := wireRecordLines(out)
	if len(wireLines) == 0 {
		t.Fatalf("no records reached the wire; out:\n%s", out)
	}

	logLines := strings.Split(strings.TrimSuffix(log, "\n"), "\n")

	logSet := make(map[string]bool, len(logLines))
	for _, l := range logLines {
		logSet[l] = true
	}

	var lastSeq uint64

	for _, line := range wireLines {
		r, ok := guard.ParseRecordLine(line, cfg.key)
		if !ok {
			t.Fatalf("wire line is not a valid record: %q", line)
		}

		if !logSet[line] {
			t.Fatalf("wire frame missing from the log (log-then-wire violated): %q", line)
		}

		if r.Seq <= lastSeq {
			t.Fatalf("wire frames must arrive in record order: seq %d after %d", r.Seq, lastSeq)
		}

		lastSeq = r.Seq
	}
}

// wireRecordLines extracts the record lines from a wire stream.
func wireRecordLines(out string) []string {
	var lines []string

	for l := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.HasPrefix(l, guard.RecordPrefix) {
			lines = append(lines, l)
		}
	}

	return lines
}

// TestWireCarriesNarrative pins spec 9.1: the live stream is everything the
// guardian writes to the log, child output included; free-form lines ride
// the wire beside the records and stay out of the record stream.
func TestWireCarriesNarrative(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", "echo ACTIVATION-NARRATIVE\nexit 0\n")}}

	h := newWireHarness(t, cfg)
	st, log, out := h.run(nil)

	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	if !strings.Contains(log, "ACTIVATION-NARRATIVE") {
		t.Fatalf("child output must land in the log; log:\n%s", log)
	}

	if !strings.Contains(out, "ACTIVATION-NARRATIVE") {
		t.Fatalf("child output must ride the wire (spec 9.1); out:\n%s", out)
	}

	for _, line := range wireRecordLines(out) {
		if strings.Contains(line, "ACTIVATION-NARRATIVE") {
			t.Fatalf("narrative must not be wrapped in records: %q", line)
		}
	}
}

// gatedWriter blocks every Write until released and counts the writes that
// actually happened, so the drop accounting is exact regardless of goroutine
// scheduling: every frame is either dropped or written, never both.
type gatedWriter struct {
	started    chan struct{}
	release    chan struct{}
	signalOnce sync.Once

	mu     sync.Mutex
	writes int
	frames []string
}

func (w *gatedWriter) Write(p []byte) (int, error) {
	w.signalOnce.Do(func() { close(w.started) })
	<-w.release

	w.mu.Lock()
	w.writes++
	w.frames = append(w.frames, string(p))
	w.mu.Unlock()

	return len(p), nil
}

// TestWireDropOldest pins the bounded drop-oldest buffer: beyond the buffer
// the oldest frames yield, every frame is either dropped or written (never
// both, never lost silently), the written frames stay in order, and every
// drop is reported as LINK_DEGRADED in the log.
func TestWireDropOldest(t *testing.T) {
	gate := &gatedWriter{started: make(chan struct{}), release: make(chan struct{})}

	var (
		mu    sync.Mutex
		drops int
	)

	dir := t.TempDir()

	logw, err := OpenLogWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logw.Close()

	ww := newWireWriter(gate,
		func() {
			mu.Lock()
			drops++
			mu.Unlock()
			logw.WriteLine(guard.FormatRecord(guard.Record{K: "k", W: guard.WriterGuardian, Seq: 1, TS: 1, Ev: guard.EventLinkDegraded, Rs: "drop"}))
		},
		nil)

	total := wireBufferFrames + 10

	for i := range total {
		ww.send(fmt.Sprintf("frame-%03d", i))
	}

	// The state machine analog: send never blocked, so the loop above
	// completed while the writer is still blocked on its first frame.
	<-gate.started

	mu.Lock()
	gotDrops := drops
	mu.Unlock()

	if gotDrops == 0 {
		t.Fatalf("a full buffer must drop frames, got 0")
	}

	close(gate.release) // unblock the writer goroutine

	ww.close()

	gate.mu.Lock()
	writes, frames := gate.writes, append([]string(nil), gate.frames...)
	gate.mu.Unlock()

	if writes+gotDrops != total {
		t.Fatalf("every frame must be dropped or written: writes=%d drops=%d total=%d", writes, gotDrops, total)
	}

	last := ""
	for _, f := range frames {
		if f <= last {
			t.Fatalf("written frames must stay in order: %q after %q", f, last)
		}

		last = f
	}

	newest := fmt.Sprintf("frame-%03d\n", total-1)
	if last != newest {
		t.Fatalf("the newest frame must survive the drop-oldest: last=%q want=%q", last, newest)
	}

	f, err := guard.ScanFile(dir+"/"+logName, "k")
	if err != nil || !f.Found || f.Last.Ev != guard.EventLinkDegraded {
		t.Fatalf("drops must be logged as LINK_DEGRADED: %+v err=%v", f, err)
	}
}

// TestWireDropOldestLogOnly pins that the drop report lands in the log and
// never on the wire (spec 6.1: LINK_DEGRADED is a log-only record).
func TestWireDropOldestLogOnly(t *testing.T) {
	dir := t.TempDir()

	gate := &gatedWriter{started: make(chan struct{}), release: make(chan struct{})}

	logw, err := OpenLogWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logw.Close()

	var seq uint64

	ww := newWireWriter(gate, nil, nil)
	ww.setHooks(
		func() {
			logw.WriteLine(guard.FormatRecord(guard.Record{K: "k", W: guard.WriterGuardian, Seq: seq + 1, TS: 1, Ev: guard.EventLinkDegraded, Rs: "drop"}))
		},
		nil,
	)

	for i := range wireBufferFrames + 5 {
		seq++

		ww.send(fmt.Sprintf("frame-%d", i))
	}

	close(gate.release)
	ww.close()

	log := readLog(t, dir)
	if !strings.Contains(log, guard.EventLinkDegraded) {
		t.Fatalf("drops must be logged as LINK_DEGRADED; log:\n%s", log)
	}
}

// TestWireLinkDownLogOnly pins the LINK_DOWN path: command-pipe EOF logs
// LINK_DOWN, and the record never appears on the event wire.
func TestWireLinkDownLogOnly(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)
	cfg.activationTimeout = 2 * time.Second

	h := newWireHarness(t, cfg)
	// Close the command pipe's write end right away: the guardian's reader
	// sees EOF (the relay is gone) and logs LINK_DOWN while the transaction
	// proceeds to its terminal state on its own.
	go func() {
		_ = h.cmdW.Close()
	}()

	st, log, out := h.run(nil)

	if st != stateCommitted {
		t.Fatalf("transaction must proceed with a dead wire: state=%s log:\n%s", st, log)
	}

	if !strings.Contains(log, guard.EventLinkDown) {
		t.Fatalf("command pipe EOF must log LINK_DOWN; log:\n%s", log)
	}

	if strings.Contains(out, guard.EventLinkDown) {
		t.Fatalf("LINK_DOWN must never be announced on the wire; out:\n%s", out)
	}
}

// TestWireConfirmRidCorrelation pins the rid contract end to end: a wire
// confirm acks with the same rid; signal-driven acks carry none.
func TestWireConfirmRidCorrelation(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 5 * time.Second

	h := newWireHarness(t, cfg)

	st, log, _ := h.run(func(w io.Writer) {
		writeFrame(w, "confirm", 7)
	})

	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventConfirmConsumed)
	if !ok {
		t.Fatalf("no CONFIRM_CONSUMED record; log:\n%s", log)
	}

	if m.Rid != 7 {
		t.Fatalf("CONFIRM_CONSUMED must carry the command rid, got %d", m.Rid)
	}
}

// TestWireRevertRidDuringActivation pins the revert ack: a wire revert during
// activation surfaces on the REVERT_START record with the matching rid.
func TestWireRevertRidDuringActivation(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)
	cfg.activationArgv = [][]string{{writeStub(t, stubs, "activate", "sleep 30\n")}}

	h := newWireHarness(t, cfg)

	st, log, _ := h.run(func(w io.Writer) {
		time.Sleep(150 * time.Millisecond) // let the activation start
		writeFrame(w, "revert", 3)
	})

	if st != stateReverted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventRevertStart)
	if !ok {
		t.Fatalf("no REVERT_START record; log:\n%s", log)
	}

	if m.Rid != 3 {
		t.Fatalf("REVERT_START must carry the wire revert rid, got %d", m.Rid)
	}
}

// TestWireUnknownCommands pins the drain-forever rule: unknown and malformed
// command lines produce ack-shaped records without a rid (never acks) and the
// transaction keeps consuming valid frames after them.
func TestWireUnknownCommands(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 5 * time.Second

	h := newWireHarness(t, cfg)

	st, log, _ := h.run(func(w io.Writer) {
		_, _ = io.WriteString(w, "{\"c\":\"frobnicate\",\"rid\":9}\n")
		_, _ = io.WriteString(w, "not json at all\n")
		_, _ = io.WriteString(w, "{\"c\":\"\"}\n")
		writeFrame(w, "confirm", 11)
	})

	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecordRs(t, slot, cfg.key, guard.EventLateRequestAck, "unknown command")
	if !ok {
		t.Fatalf("unknown command must be recorded; log:\n%s", log)
	}

	if m.Rid != 0 {
		t.Fatalf("an unknown-command record must never carry a rid (never an ack): %d", m.Rid)
	}

	if _, ok = findRecordRs(t, slot, cfg.key, guard.EventLateRequestAck, "malformed command frame"); !ok {
		t.Fatalf("malformed line must be recorded; log:\n%s", log)
	}

	m, ok = findRecord(t, slot, cfg.key, guard.EventConfirmConsumed)
	if !ok || m.Rid != 11 {
		t.Fatalf("a valid frame after bad ones must still ack: ok=%v rid=%d", ok, m.Rid)
	}
}

// findRecordRs returns the first record of event whose rs contains want.
func findRecordRs(t *testing.T, dir, key, event, want string) (guard.Record, bool) {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(dir, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	for line := range strings.SplitSeq(string(b), "\n") {
		r, ok := guard.ParseRecordLine(line, key)
		if ok && r.Ev == event && strings.Contains(r.Rs, want) {
			return r, true
		}
	}

	return guard.Record{}, false
}

// TestWirePreHelloDrain pins the drain-from-before-HELLO rule: a command
// frame written before the state machine runs is queued, not lost, and is
// consumed at ACTIVATED (same semantics as the pre-HELLO signal rules).
func TestWirePreHelloDrain(t *testing.T) {
	slot, stubs := t.TempDir(), t.TempDir()
	cfg := wireAutoCfg(t, slot, stubs)
	cfg.confirmation = gateMagic
	cfg.confirmTimeout = 5 * time.Second

	h := newWireHarness(t, cfg)
	// Feed BEFORE run(): the frame lands in the pipe before HELLO exists.
	writeFrame(h.cmdW, "confirm", 21)

	st, log, _ := h.run(nil)

	if st != stateCommitted {
		t.Fatalf("state=%s log:\n%s", st, log)
	}

	m, ok := findRecord(t, slot, cfg.key, guard.EventConfirmConsumed)
	if !ok || m.Rid != 21 {
		t.Fatalf("pre-HELLO confirm must not be lost: ok=%v rid=%d; log:\n%s", ok, m.Rid, log)
	}
}

// TestRunGuardedRecoversPanic pins the top-level recover: the panic surfaces
// as a terminal error record (log first, then wire best-effort) and the
// caller exits nonzero.
func TestRunGuardedRecoversPanic(t *testing.T) {
	slot := t.TempDir()

	lock, err := openLockedLog(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer lock.Close()

	logw, err := OpenLogWriter(slot)
	if err != nil {
		t.Fatal(err)
	}

	defer logw.Close()

	var seq uint64

	core := &txCore{key: "k", writer: guard.WriterGuardian, logw: logw, seqMu: &sync.Mutex{}, seq: &seq}

	_, panicked := runGuarded(
		func() txState { panic("boom in the machine") },
		func(err error) {
			core.emit(guard.EventExit, guard.Record{St: string(stateFailedPrecondition), RC: 1, Err: err.Error()})
		},
	)

	if !panicked {
		t.Fatal("runGuarded must report the panic")
	}

	f, err := guard.ScanFile(slot+"/"+logName, "k")
	if err != nil || !f.Found || !f.Terminal || f.Last.Ev != guard.EventExit || f.Last.RC != 1 {
		t.Fatalf("panic must leave a terminal error record: %+v err=%v", f, err)
	}

	if !strings.Contains(f.Err, "boom in the machine") {
		t.Fatalf("the panic text must survive in the excerpt: %q", f.Err)
	}
}

// TestWireFrameJSON pins the frame shape shared with the deployer.
func TestWireFrameJSON(t *testing.T) {
	if got := formatTestFrame("confirm", 4); got != "{\"c\":\"confirm\",\"rid\":4}\n" {
		t.Fatalf("frame shape drifted: %q", got)
	}

	var frame wireCommandFrame

	err := json.Unmarshal([]byte(`{"c":"revert","rid":9}`), &frame)
	if err != nil || frame.C != "revert" || frame.Rid != 9 {
		t.Fatalf("frame parse drifted: %+v err=%v", frame, err)
	}
}
