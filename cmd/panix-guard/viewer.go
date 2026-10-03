package main

import (
	"flag"
	"fmt"
	"github.com/mihakrumpestar/panix/internal/guard"
	"io"
	"os"
	"strings"
	"time"
)

// Viewer tuning knobs are variables so tests can shrink the timings.
var (
	viewerPoll      = 200 * time.Millisecond
	viewerHelloWait = 30 * time.Second
)

// runAttach implements the reconnect viewer (spec 9.2): it tails the slot log
// and streams it to stdout, resuming from a byte offset, and maps the
// terminal state to the guardian outcome vocabulary (spec 9.5).
func runAttach(args []string) error {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	dir := fs.String("dir", "", "slot directory")
	key := fs.String("key", "", "per-deploy key (deploy nonce)")

	from := fs.Int64("from", 0, "byte offset to resume from")

	err := fs.Parse(args)
	if err != nil {
		return err
	}

	if *dir == "" || *key == "" {
		return errAttachUsage
	}

	os.Exit(followLog(*dir, *key, *from))

	return nil
}

// followLog tails the slot log from offset until a terminal marker (mapped
// exit code) or until the guardian disappears without one (ExitCancelled:
// panix then converges via converge, spec 9.4). A file smaller than the
// offset means the log was front-truncated: restart at 0 and let seq dedupe
// suppress replays (spec 9.2).
func followLog(dir, key string, offset int64) int {
	path := slotLogPath(dir)
	start := time.Now()
	lastSeq := uint64(0)
	guardianPID := 0

	var partial string

	readOff := offset

	for {
		lines, err := readNewLines(path, &readOff, &partial)
		if err != nil {
			// A vanished slot (user-tier /run/user removal, tmpfs clear) cannot be
			// tailed; the outcome is unknown and convergence stays with converge.
			return guard.ExitCancelled
		}

		for _, line := range lines {
			fmt.Println(line) // raw stream: child output and records alike (spec 9.1)

			r, ok := guard.ParseRecordLine(line, key)
			if !ok {
				continue
			}

			if lastSeq > 0 && r.Seq <= lastSeq {
				continue // replay after a truncation restart (spec 9.2)
			}

			lastSeq = r.Seq
			switch r.Ev {
			case guard.EventCommitted:
				return guard.ExitCommitted
			case guard.EventReverted:
				return guard.ExitReverted
			case guard.EventRevertFailed:
				return guard.ExitRevertFailed
			case guard.EventFailedPrecondition:
				return guard.ExitFailedPrecondition
			case guard.EventExit:
				if r.St == string(stateActivationExited) {
					return guard.ExitActivationExited
				}
			case guard.EventHello, guard.EventState:
				if r.PID > 0 {
					guardianPID = r.PID
				}
			}
		}
		// Guardian-death detection (spec 9.1): a guardian that disappeared without a
		// terminal marker leaves the outcome unknown.
		if guardianPID > 0 && !guardianAlive(guardianPID, key) {
			return guard.ExitCancelled
		}

		if !helloSeen(guardianPID) && time.Since(start) > viewerHelloWait {
			return guard.ExitCancelled
		}

		time.Sleep(viewerPoll)
	}
}

// helloSeen reports whether a guardian identity has been observed yet.
func helloSeen(guardianPID int) bool { return guardianPID > 0 }

// readNewLines returns the complete lines appended to path since *offset, carrying the
// trailing partial line across calls in *partial. When the file shrank below the
// offset (front-truncation) the read restarts at 0; marker dedupe is the caller's job.
func readNewLines(path string, offset *int64, partial *string) ([]string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if st.Size() < *offset {
		*offset = 0
		*partial = ""
	}

	if st.Size() == *offset {
		return nil, nil
	}

	f, err := os.Open(path) //nolint:gosec // the path is the caller-chosen slot's log by contract
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, st.Size()-*offset)

	n, rerr := f.ReadAt(buf, *offset)
	if rerr != nil && rerr != io.EOF {
		return nil, rerr
	}

	buf = buf[:n]
	*offset += int64(n)
	data := *partial + string(buf)
	*partial = ""

	lines := strings.Split(data, "\n")
	if !strings.HasSuffix(data, "\n") && len(lines) > 0 {
		// The trailing segment has no newline yet: carry it over to the next read.
		*partial = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	}

	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			out = append(out, l)
		}
	}

	return out, nil
}
