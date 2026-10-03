package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Slot log names (spec 10.1): the slot holds exactly two entries, the log and the
// gc-root symlink.
const (
	logName     = "log"
	gcRootName  = "gc-root"
	logCapBytes = 4 << 20  // 4MiB cap (spec 7)
	logTailKeep = 64 << 10 // 64KiB minimum retained tail (spec 7)
)

// LogWriter is the single serialized writer for the slot log. The guardian (and
// post-mortem converge) write child output lines and marker lines through it so lines
// never interleave mid-line (spec 6.4) and the 4MiB cap policy stays in control.
// Never redirect a child's stdio straight into the log fd: writes must flow through
// this writer.
type LogWriter struct {
	mu   sync.Mutex
	f    *os.File
	size int64
}

// slotLogPath is the slot's log path (the one filesystem entry every verb
// resolves).
func slotLogPath(dir string) string { return filepath.Join(dir, logName) }

// OpenLogWriter opens (or creates) the slot log for appending and positions the write
// offset at the current end of file.
func OpenLogWriter(dir string) (*LogWriter, error) {
	// The slot directory is operator-provided by design: the guardian runs on
	// the target the caller chose, and the log path is always <slot>/log
	// under that directory. No traversal surface beyond the caller's own
	// choice, so the variable path is contract, not vulnerability.
	f, err := os.OpenFile(slotLogPath(dir), os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // slot paths are the caller's own choice (spec 10.1); the child name is fixed
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errLogUnwritable, err)
	}

	st, err := f.Stat()
	if err != nil {
		f.Close()

		return nil, fmt.Errorf("stat slot log: %w", err)
	}

	if _, err := f.Seek(st.Size(), io.SeekStart); err != nil {
		f.Close()

		return nil, fmt.Errorf("seek slot log: %w", err)
	}

	return &LogWriter{f: f, size: st.Size()}, nil
}

// WriteLine appends one line (a newline is added). Best effort per spec 6.4: write
// errors are swallowed and never abort the transaction.
func (w *LogWriter) WriteLine(line string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.f.WriteString(line + "\n"); err != nil {
		return
	}

	w.size += int64(len(line) + 1)
	w.truncateFront()
}

// truncateFront enforces the cap policy (spec 7): past 4MiB, rewrite the file with the
// last >= 64KiB aligned to a complete line. Single writer under the mutex; a crash
// mid-rewrite tears the final line, which parsers ignore and the next heartbeat repairs.
//
// PINNED INVARIANT: truncation MUST stay in-place (Truncate + WriteAt on the same
// inode). Rename-based truncation (write a fresh file and rename it over the log) is
// PROHIBITED: the rename forks the inode out from under every inherited file
// description, so the live guardian's lock OFD and its stdio alias keep appending to
// the orphaned inode while every new reader follows the replacement file, silently
// forking the transaction record and the lock. TestLogWriterTruncateKeepsInode pins
// this.
func (w *LogWriter) truncateFront() {
	if w.size <= logCapBytes {
		return
	}

	keep := min(int64(logTailKeep), w.size)

	buf := make([]byte, keep)

	n, err := w.f.ReadAt(buf, w.size-keep)
	if err != nil && err != io.EOF {
		return
	}

	buf = buf[:n]

	idx := bytes.IndexByte(buf, '\n')
	if idx < 0 {
		// No complete line in the tail; skip this round and retry on the next write.
		return
	}

	aligned := buf[idx+1:]

	if err := w.f.Truncate(0); err != nil {
		return
	}

	if _, err := w.f.WriteAt(aligned, 0); err != nil {
		return
	}

	if _, err := w.f.Seek(int64(len(aligned)), io.SeekStart); err != nil {
		return
	}

	w.size = int64(len(aligned))
}

// TruncateInPlace clears the log on the same inode (spec 7: the start sweep's
// fresh-transaction reset; rename-based truncation is prohibited). The
// writer's size bookkeeping rewinds with the file.
func (w *LogWriter) TruncateInPlace() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.f.Truncate(0)
	if err != nil {
		return fmt.Errorf("truncate slot log: %w", err)
	}

	if _, err := w.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind slot log: %w", err)
	}

	w.size = 0

	return nil
}

// Close releases the log file.
func (w *LogWriter) Close() error { return w.f.Close() }
