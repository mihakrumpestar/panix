package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/mihakrumpestar/panix/internal/guard"
)

// inspect verdict (spec 9.5): one JSON object on stdout, nothing else. The
// verb absorbs the legacy lock probe: the lock state is derivable even from a
// slot whose log carries no parsable v2 records (legacy @PG1 slots, garbage),
// which the deployer's legacy sweep relies on.
type inspectVerdict struct {
	Status   string        `json:"status"`
	Terminal bool          `json:"terminal"`
	Old      string        `json:"old,omitempty"`
	New      string        `json:"new,omitempty"`
	Gen      int64         `json:"gen,omitempty"`
	Mode     string        `json:"mode,omitempty"`
	Tier     string        `json:"tier,omitempty"`
	RC       int           `json:"rc,omitempty"`
	Err      string        `json:"err,omitempty"`
	Key      string        `json:"key,omitempty"`
	Lock     bool          `json:"lock"`
	Txn      *txnEmbedding `json:"txn,omitempty"`
}

// txnEmbedding is the TXN record's embedded step lists, verbatim.
type txnEmbedding struct {
	AA string `json:"aa,omitempty"`
	CA string `json:"ca,omitempty"`
	RA string `json:"ra,omitempty"`
	IV string `json:"iv,omitempty"`
}

// runInspect prints one JSON verdict for the slot (spec 9.5) and exits with
// the inspect exit code: 0 verdict printed, 1 log unparseable beyond noise
// (the verdict still reports the lock state), 4 slot or log missing.
func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)

	dir := fs.String("dir", "", "slot directory")

	err := fs.Parse(args)
	if err != nil {
		return err
	}

	if *dir == "" {
		return errInspectUsage
	}

	os.Exit(inspectRun(*dir))

	return nil
}

// inspectRun derives the verdict and returns the process exit code.
func inspectRun(dir string) int {
	path := slotLogPath(dir)

	if _, err := os.Stat(path); err != nil {
		return guard.InspectExitNoSlot // no slot or no log: no lock can exist
	}

	lock := probeSlotLockState(path)

	verdict, unparseable := inspectFold(path)
	verdict.Lock = lock

	b, err := json.Marshal(verdict)
	if err != nil {
		// The verdict struct carries only marshallable kinds; this is
		// unreachable, and a silent stdout is worse than a stderr note.
		fmt.Fprintf(os.Stderr, "panix-guard: marshal verdict: %v\n", err)

		return guard.InspectExitUnparseable
	}

	fmt.Println(string(b))

	if unparseable {
		return guard.InspectExitUnparseable
	}

	return guard.InspectExitOK
}

// probeSlotLockState answers "is a live transaction holding the log lock"
// with a fresh open plus a transient nonblocking flock (spec 10.1: the
// never-LOCK_UN rule binds only inherited fds; a fresh open file description
// releases by closing, no explicit unlock needed).
func probeSlotLockState(path string) bool {
	f, err := os.OpenFile(path, os.O_RDWR, 0o600) //nolint:gosec // the path is the caller-chosen slot's log by contract
	if err != nil {
		return false // unreadable log: no lock state is derivable
	}

	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return true // held: a live transaction owns the slot
	}

	return false // the fresh grab succeeded: free (closing releases it)
}

// inspectFold parses the log tail and folds the last writer's transaction.
// unparseable is true when the tail carries sentinel lines but none of them
// is a valid v2 record: a legacy @PG1 slot ("legacy") or broken @PG2 lines
// ("unknown"). The lock state stays valid either way.
func inspectFold(path string) (inspectVerdict, bool) {
	lines, err := readTailLines(path)
	if err != nil {
		return inspectVerdict{Status: "unknown"}, true
	}

	var (
		valid     []guard.Record
		legacy    bool
		brokenPG2 bool
		txn       *txnEmbedding
	)

	for _, line := range lines {
		if r, ok := guard.ParseRecordLine(line, ""); ok {
			valid = append(valid, r)

			if r.Ev == guard.EventTxn {
				txn = &txnEmbedding{AA: r.AA, CA: r.CA, RA: r.RA, IV: r.IV}
			}

			continue
		}

		switch {
		case strings.HasPrefix(line, "@PG1 "):
			legacy = true
		case strings.HasPrefix(line, guard.RecordPrefix):
			brokenPG2 = true
		}
	}

	if len(valid) == 0 {
		switch {
		case legacy:
			return inspectVerdict{Status: "legacy"}, true
		case brokenPG2:
			return inspectVerdict{Status: "unknown"}, true
		default:
			return inspectVerdict{}, false // no sentinel lines: nothing recorded yet
		}
	}

	folded := guard.Fold(valid)

	verdict := inspectVerdict{
		Status:   folded.Status,
		Terminal: folded.Terminal,
		Old:      folded.Old,
		New:      folded.New,
		Gen:      folded.Gen,
		Mode:     folded.Mode,
		Tier:     folded.Tier,
		RC:       folded.RC,
		Err:      folded.Err,
		Key:      folded.Key,
		Txn:      txn,
	}

	return verdict, false
}

// readTailLines reads the last complete lines of the log within the scan
// window (the same tolerance as the scanners: a torn trailing line is noise).
func readTailLines(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // the path is the caller-chosen slot's log by contract
	if err != nil {
		return nil, err
	}

	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := st.Size()
	if size == 0 {
		return nil, nil
	}

	start := max(size-guard.LogTailScanBytes, 0)
	buf := make([]byte, size-start)

	n, err := f.ReadAt(buf, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	buf = buf[:n]

	idx := strings.LastIndexByte(string(buf), '\n')
	if idx < 0 {
		return nil, nil // one torn line only: no complete lines
	}

	var lines []string

	for line := range strings.SplitSeq(string(buf[:idx]), "\n") {
		if line != "" {
			lines = append(lines, strings.TrimSuffix(line, "\r"))
		}
	}

	return lines, nil
}
