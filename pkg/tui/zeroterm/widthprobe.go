// Derived from charm.land/bubbletea/v2. See pkg/tui/LICENSE.charmbracelet.

package zeroterm

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Width probe protocol: one byte stream, written in a single call and read in a
// single pass, so startup pays one round trip. A synchronized output update
// (DEC 2026) wraps one cursor position query per glyph: move to column one,
// write the glyph, ask for the position (\x1b[6n). The reply ESC [ row ; col R
// gives width col-1, and the terminal processes the stream in order, so the Nth
// reply belongs to the Nth query. The trailing erase clears the probed line on
// terminals that ignore synchronized output.
const (
	probeCPRRequest = "\x1b[6n"
	probeSyncBegin  = "\x1b[?2026h"
	probeSyncEnd    = "\x1b[?2026l"
	probeEraseLine  = "\r\x1b[K"
	probeBudget     = 250 * time.Millisecond //nolint:mnd // overall reply budget
	probeReadChunk  = 512                    //nolint:mnd // reply read buffer
)

// ErrProbeNotTerminal is returned when the input fd is not a terminal, so
// callers keep the static width table.
var ErrProbeNotTerminal = errors.New("width probe: input is not a terminal")

// ProbeGlyphWidths measures each glyph against the real terminal by asking
// where the cursor lands after writing it, returning widths keyed by glyph plus
// input bytes that were not part of a report. The caller MUST feed those
// leftover bytes to the input loop: they are user keystrokes typed while the
// probe ran. The probe never changes raw mode or the alternate screen, bails out
// with ErrProbeNotTerminal, and trusts no measurement unless the two control
// glyphs measure 1 and 2.
func (t *Terminal) ProbeGlyphWidths(glyphs []string) (map[string]int, []byte, error) {
	if t == nil || t.in == nil || t.out == nil {
		return nil, nil, ErrProbeNotTerminal
	}

	if !term.IsTerminal(int(t.in.Fd())) {
		return nil, nil, ErrProbeNotTerminal
	}

	return t.probeGlyphWidths(glyphs)
}

// probeGlyphWidths runs the probe assuming the caller validated the terminal.
// It is separate from ProbeGlyphWidths so tests can drive it over pipes.
func (t *Terminal) probeGlyphWidths(glyphs []string) (map[string]int, []byte, error) {
	targets := probeTargets(glyphs)

	writeErr := t.writeProbe(buildProbeScript(targets))
	if writeErr != nil {
		// Balance the pair in case a partial write left synchronized output open.
		_ = t.writeProbe(probeSyncEnd + probeEraseLine)

		return nil, nil, writeErr
	}

	cols, leftover := t.readProbeReplies(len(targets))

	err := verifyProbeControl(cols)
	if err != nil {
		return nil, leftover, err
	}

	measured := make(map[string]int, len(targets)-len(probeControlGlyphs))

	for index := len(probeControlGlyphs); index < len(targets); index++ {
		if index < len(cols) {
			measured[targets[index]] = cols[index] - 1
		}
	}

	return measured, leftover, nil
}

// probeControlGlyphs gate the probe and are always the first two targets: a
// plain ASCII letter must measure 1 and a CJK ideograph 2.
var probeControlGlyphs = []struct {
	glyph string
	width int
}{
	{"a", 1},
	{"\u6f22", 2},
}

// probeTargets returns the control glyphs first, then deduplicated non-ASCII
// glyphs; empty and ASCII-only glyphs are skipped.
func probeTargets(glyphs []string) []string {
	targets := make([]string, 0, len(glyphs)+len(probeControlGlyphs))
	seen := make(map[string]struct{}, len(glyphs)+len(probeControlGlyphs))

	for _, control := range probeControlGlyphs {
		targets = append(targets, control.glyph)
		seen[control.glyph] = struct{}{}
	}

	for _, glyph := range glyphs {
		if glyph == "" || isASCIIGlyph(glyph) {
			continue
		}

		if _, ok := seen[glyph]; ok {
			continue
		}

		seen[glyph] = struct{}{}
		targets = append(targets, glyph)
	}

	return targets
}

// buildProbeScript builds the single byte stream: synchronized output begin, one
// cursor position query per target, synchronized output end, and erase.
func buildProbeScript(targets []string) string {
	var script strings.Builder

	script.WriteString(probeSyncBegin)

	for _, target := range targets {
		script.WriteString("\r")
		script.WriteString(target)
		script.WriteString(probeCPRRequest)
	}

	script.WriteString(probeSyncEnd)
	script.WriteString(probeEraseLine)

	return script.String()
}

// verifyProbeControl aborts the probe on a missing or wrong control reply: the
// terminal does not implement the protocol reliably, so no measurement is
// trusted.
func verifyProbeControl(cols []int) error {
	for index, control := range probeControlGlyphs {
		if index >= len(cols) {
			return errors.Errorf("width probe: missing reply for control glyph %q", control.glyph)
		}

		if width := cols[index] - 1; width != control.width {
			return errors.Errorf(
				"width probe control glyph %q measured %d, want %d",
				control.glyph, width, control.width,
			)
		}
	}

	return nil
}

// writeProbe writes script in one call; the follow-up Sync is only a best-effort
// flush and is a no-op on a tty. Tests can redirect it through probeOut to
// observe write boundaries.
func (t *Terminal) writeProbe(script string) error {
	writer := io.Writer(t.out)

	if t.probeOut != nil {
		writer = t.probeOut
	}

	_, err := io.WriteString(writer, script)
	if err != nil {
		return errors.Wrap(err, "width probe write")
	}

	if t.probeOut == nil {
		_ = t.out.Sync()
	}

	return nil
}

// readProbeReplies reads replies under one overall budget and returns the columns
// that arrived, in order, plus bytes that were not part of a report. Missing
// replies shorten cols; the control gate decides whether that is fatal.
func (t *Terminal) readProbeReplies(want int) ([]int, []byte) {
	cols := make([]int, 0, want)

	var leftover []byte

	var buf []byte

	chunk := make([]byte, probeReadChunk)
	deadline := time.Now().Add(probeBudget)

	for len(cols) < want {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		ready, err := pollReadable(t.in, remaining)
		if err != nil {
			break
		}

		if !ready {
			continue
		}

		bytesRead, err := t.in.Read(chunk)
		if err != nil {
			break
		}

		if bytesRead == 0 {
			continue
		}

		buf = append(buf, chunk[:bytesRead]...)

		for len(cols) < want {
			col, before, after, ok := parseCPR(buf)
			if !ok {
				break
			}

			cols = append(cols, col)
			leftover = append(leftover, before...)

			buf = after
		}
	}

	leftover = trimIncompleteEscape(append(leftover, buf...))

	return cols, leftover
}

// trimIncompleteEscape drops a trailing, incomplete ANSI escape sequence so a
// probe reply fragment is never replayed as user input.
func trimIncompleteEscape(data []byte) []byte {
	start := bytes.LastIndexByte(data, '\x1b')
	if start < 0 || escapeSequenceComplete(data[start:]) {
		return data
	}

	return data[:start]
}

// ss3SequenceLen is ESC O plus one final byte.
const ss3SequenceLen = 3

// escapeSequenceComplete reports whether seq, starting with ESC, is complete.
func escapeSequenceComplete(seq []byte) bool {
	if len(seq) < 2 {
		return false
	}

	switch seq[1] {
	case '[':
		return csiComplete(seq)
	case 'O':
		return len(seq) >= ss3SequenceLen
	default:
		return true
	}
}

// csiComplete reports whether seq (ESC [ ...) has reached a final byte. X10
// mouse reports carry three payload bytes after CSI M.
func csiComplete(seq []byte) bool {
	if len(seq) >= 3 && seq[2] == 'M' {
		return len(seq) >= x10MouseLen
	}

	for index := 2; index < len(seq); index++ {
		final := seq[index]
		if final >= 0x40 && final <= 0x7e {
			return true
		}

		if final < 0x20 || final > 0x3f {
			return false
		}
	}

	return false
}

// pollReadable reports whether file is readable before the timeout elapses.
func pollReadable(file *os.File, timeout time.Duration) (bool, error) {
	millis := int(timeout.Milliseconds())
	if millis <= 0 {
		millis = 1
	}

	fds := []unix.PollFd{{Fd: int32(file.Fd()), Events: unix.POLLIN}} //nolint:gosec // fd fits in int32

	count, err := unix.Poll(fds, millis)
	if err != nil {
		if errors.Is(err, unix.EINTR) {
			return false, nil
		}

		return false, errors.Wrap(err, "width probe poll")
	}

	return count > 0, nil
}

// parseCPR finds the first cursor position report (ESC [ row ; col R) in data
// and returns its column plus the bytes before and after the report.
func parseCPR(data []byte) (int, []byte, []byte, bool) {
	for index := 0; index+1 < len(data); index++ {
		if data[index] != '\x1b' || data[index+1] != '[' {
			continue
		}

		col, end, ok := parseCPRAt(data, index)
		if !ok {
			continue
		}

		return col, data[:index], data[end:], true
	}

	return 0, nil, nil, false
}

// parseCPRAt parses a cursor position report starting at the ESC byte at start
// and returns its column plus the index just past the final R.
func parseCPRAt(data []byte, start int) (int, int, bool) {
	pos := start + 2

	pos = skipDigits(data, pos)
	if pos == start+2 || pos >= len(data) || data[pos] != ';' {
		return 0, 0, false
	}

	pos++

	colStart := pos

	pos = skipDigits(data, pos)
	if pos == colStart || pos >= len(data) || data[pos] != 'R' {
		return 0, 0, false
	}

	col, err := strconv.Atoi(string(data[colStart:pos]))
	if err != nil {
		return 0, 0, false
	}

	return col, pos + 1, true
}

// skipDigits returns the position after the run of ASCII digits at pos.
func skipDigits(data []byte, pos int) int {
	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		pos++
	}

	return pos
}

// isASCIIGlyph reports whether every byte of glyph is ASCII; those always
// occupy one cell, so the probe skips them.
func isASCIIGlyph(glyph string) bool {
	for i := range len(glyph) {
		if glyph[i] >= 0x80 { //nolint:mnd // high byte = non-ASCII
			return false
		}
	}

	return true
}
