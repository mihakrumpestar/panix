package main

import (
	"fmt"
	"github.com/mihakrumpestar/panix/internal/guard"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// txCore is the shared convergence core for the live transaction and the post-mortem
// converge (spec 9.3): the profile rules (restore when pointing at NEW, generation
// cleanup of the failed generation) are runtime-derived and identical for both.
type txCore struct {
	profilePath     string
	nixEnv          string
	oldClosure      string
	newClosure      string
	gen             int64
	mode            string
	commitArgv      [][]string
	revertArgv      [][]string
	invariantTarget string
	key             string
	writer          string // record writer tag: WriterGuardian | WriterDecide
	logw            *LogWriter
	wire            *wireWriter  // nil when there is no event pipe (log-only)
	onLine          func(string) // optional excerpt recorder (live transaction only)

	// lastStep* carries the most recent failing panix-composed step for the
	// terminal records' error fidelity (spec 7): a failed commit or revert
	// must name its own failing step's argv and error, never a stale phase
	// error from an earlier phase. Sequential access: runStep and the
	// terminal emitters run on one goroutine per transaction (the converge
	// command owns its own core instance).
	lastStepArgv []string
	lastStepErr  error

	seqMu *sync.Mutex
	seq   *uint64
}

// emit writes one sentinel record under the shared sequence counter: one
// counter continues across every writer of the log, never restarting (spec 7).
// Dual-write ordering (spec 6.1): the record is serialized once, lands in the
// log first, then joins the wire queue (drop-oldest, never blocking). The
// counter increment and the log write share the lock so the log order always
// matches the sequence order even when emitters race.
func (c *txCore) emit(ev string, extra guard.Record) {
	c.seqMu.Lock()
	*c.seq++
	extra.K, extra.W, extra.Seq, extra.TS, extra.Ev = c.key, c.writer, *c.seq, time.Now().Unix(), ev
	line := guard.FormatRecord(extra)
	c.logw.WriteLine(line)
	c.seqMu.Unlock()

	if c.wire != nil {
		c.wire.send(line)
	}
}

// writeLine appends one free-form narrative line (child output, step output,
// check output, diagnostics) to the log and the wire alike: the log is
// free-form child output plus record lines (spec 7), and the live stream is
// everything the guardian writes to the log (spec 9.1). Records go through
// emit instead: they carry the envelope and the log-then-wire invariant.
func (c *txCore) writeLine(line string) {
	c.logw.WriteLine(line)

	if c.wire != nil {
		c.wire.send(line)
	}
}

// emitLogOnly writes one record to the log without touching the wire: the
// link-health records (LINK_DOWN, LINK_DEGRADED) must never be announced on
// the channel they describe (spec 6.1).
func (c *txCore) emitLogOnly(ev string, extra guard.Record) {
	c.seqMu.Lock()
	*c.seq++
	extra.K, extra.W, extra.Seq, extra.TS, extra.Ev = c.key, c.writer, *c.seq, time.Now().Unix(), ev
	c.logw.WriteLine(guard.FormatRecord(extra))
	c.seqMu.Unlock()
}

// execStep runs one panix-composed step with the conservative PATH (spec 6.4) and
// returns its combined output.
func execStep(step []string) ([]byte, error) {
	// The step argv is panix-composed by contract (spec 6.5): executing it is
	// the guardian's purpose, the caller owns the trust boundary.
	cmd := exec.Command(step[0], step[1:]...) //nolint:gosec // panix-composed argv is the contract (spec 6.5)
	cmd.Env = envWithPath(os.Environ(), conservativePath)

	return cmd.CombinedOutput()
}

// splitLines splits combined output into non-empty lines.
func splitLines(s string) []string {
	var out []string

	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}

// runStep executes one panix-composed step with its output streamed into the
// log and the wire (spec 9.1: the live stream is the log's narrative). A
// failing step records its own argv and error for the terminal records
// (spec 7 error fidelity) and narrates the failure with the errno class, so
// fork/exec-level failures that never start a child still leave the step's
// identity in the log.
func (c *txCore) runStep(step []string) int {
	out, err := execStep(step)
	for _, line := range splitLines(string(out)) {
		c.writeLine(line)

		if c.onLine != nil {
			c.onLine(line)
		}
	}

	if err != nil {
		c.lastStepArgv, c.lastStepErr = step, err
		c.writeLine(fmt.Sprintf("guard: step %s failed: %v", formatStepArgv(step), err))

		return exitCodeOf(err)
	}

	return 0
}

// stepFailure renders the last failing panix-composed step's argv and error
// for the terminal records (spec 7): empty when no step failed, so callers
// fall back to the phase-level excerpt (the failure predates the steps).
func (c *txCore) stepFailure() string {
	if c.lastStepErr == nil {
		return ""
	}

	return fmt.Sprintf("step %s: %v", formatStepArgv(c.lastStepArgv), c.lastStepErr)
}

// formatStepArgv renders a step argv as one bracketed, quoted word list: the
// failing step's identity in log lines and record excerpts.
func formatStepArgv(step []string) string {
	quoted := make([]string, len(step))
	for i, word := range step {
		quoted[i] = strconv.Quote(word)
	}

	return "[" + strings.Join(quoted, " ") + "]"
}

// invariantOK verifies the profile resolves to the expected target after the
// transaction's effects (spec 6.5). An empty target skips the check.
func (c *txCore) invariantOK() bool {
	if c.invariantTarget == "" {
		return true
	}

	got, _, err := guard.ProfileTarget(c.profilePath)
	if err != nil {
		c.writeLine(fmt.Sprintf("guard: invariant probe failed: %v", err))

		return false
	}

	return got == c.invariantTarget
}

// convergeCommit runs the commit transaction (spec 4.2): COMMIT_START before any step
// (crash-resume ordering), the panix-composed steps, then the invariant.
func (c *txCore) convergeCommit() bool {
	c.emit(guard.EventCommitStart, guard.Record{})

	for _, step := range c.commitArgv {
		if rc := c.runStep(step); rc != 0 {
			c.writeLine(fmt.Sprintf("guard: commit step failed (rc=%d)", rc))

			return false
		}
	}

	return c.invariantOK()
}

// convergeRevert executes the revert core (spec 6.3, 6.5): capture the failed
// generation before any restore re-points the profile, restore OLD when the profile
// points at NEW (pre-start set in boot mode or a partial commit), run the
// panix-composed revert steps with STC-lock backoff, then delete the failed generation
// (never the current one, verified V6). Returns true when the profile is back on OLD.
func (c *txCore) convergeRevert() bool {
	failedGen := int64(0)

	if c.profilePath != "" {
		if _, gen, err := guard.ProfileTarget(c.profilePath); err == nil && gen > c.gen {
			failedGen = gen
		}
	}

	if c.profilePath != "" {
		if cur, _, err := guard.ProfileTarget(c.profilePath); err == nil && cur == c.newClosure {
			step := []string{c.nixEnv, "-p", c.profilePath, "--set", c.oldClosure}
			if rc := c.runStep(step); rc != 0 {
				return false
			}
		}
	}

	for attempt := 0; ; attempt++ {
		allOK := true

		for _, step := range c.revertArgv {
			if rc := c.runStep(step); rc != 0 {
				allOK = false

				break
			}
		}

		if allOK {
			break
		}

		if attempt >= revertRetries {
			return false
		}

		time.Sleep(revertBackoff)
	}

	if failedGen > 0 {
		step := []string{c.nixEnv, "-p", c.profilePath, "--delete-generations", strconv.FormatInt(failedGen, 10)}
		if rc := c.runStep(step); rc != 0 {
			c.writeLine("guard: generation cleanup failed; the failed generation stays in the list")
		}
	}

	return true
}

// profileIs reports whether the profile currently resolves to storePath.
func (c *txCore) profileIs(storePath string) bool {
	if c.profilePath == "" {
		return false
	}

	got, _, err := guard.ProfileTarget(c.profilePath)

	return err == nil && got == storePath
}
