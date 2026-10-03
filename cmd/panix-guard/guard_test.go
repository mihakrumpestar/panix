package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mihakrumpestar/panix/internal/guard"
)

func TestLogWriterCapBurst(t *testing.T) {
	dir := t.TempDir()

	w, err := OpenLogWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	w.WriteLine("first line")

	lastSeq := uint64(0)

	for i := range 5000 {
		w.WriteLine(strings.Repeat("x", 1023))

		if i%500 == 0 {
			lastSeq++
			w.WriteLine(rec("k", lastSeq, guard.EventState, guard.Record{
				St: "activating",
			}))
		}
	}
	// Terminal-state records are the last writes in a real transaction; the retained
	// tail must keep the most recent one.
	lastSeq++
	w.WriteLine(rec("k", lastSeq, guard.EventState, guard.Record{
		St: "activating",
	}))
	w.Close()

	st, err := os.Stat(filepath.Join(dir, logName))
	if err != nil {
		t.Fatal(err)
	}

	if st.Size() > logCapBytes {
		t.Fatalf("cap not enforced: %d bytes on disk", st.Size())
	}

	if st.Size() < logTailKeep/2 {
		t.Fatalf("tail retained too little: %d bytes", st.Size())
	}

	f, err := guard.ScanFile(filepath.Join(dir, logName), "k")
	if err != nil || !f.Found || f.Last.Seq != lastSeq {
		t.Fatalf("last record lost across truncation: found=%v err=%v last=%+v", f.Found, err, f.Last)
	}
}

// TestLogWriterTruncateKeepsInode pins the pinned invariant on truncateFront:
// the cap rewrite stays in-place on the same inode. Rename-based truncation is
// PROHIBITED (it forks the inode out from under every inherited fd: the lock
// OFD and the stdio alias keep the orphaned inode alive while readers follow
// the replacement file), so the inode must survive the cap enforcement.
func TestLogWriterTruncateKeepsInode(t *testing.T) {
	dir := t.TempDir()

	w, err := OpenLogWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	before, err := os.Stat(filepath.Join(dir, logName))
	if err != nil {
		t.Fatal(err)
	}

	for range 5000 {
		w.WriteLine(strings.Repeat("x", 1023))
	}

	after, err := os.Stat(filepath.Join(dir, logName))
	if err != nil {
		t.Fatal(err)
	}

	if !os.SameFile(before, after) {
		t.Fatal("cap truncation must stay in-place: the log inode changed")
	}
}

func TestLogWriterSerialization(t *testing.T) {
	dir := t.TempDir()

	w, err := OpenLogWriter(dir)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for g := range 20 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			for i := range 50 {
				w.WriteLine(fmt.Sprintf("g%d-l%d payload", g, i))
			}
		}(g)
	}

	wg.Wait()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, logName)) //nolint:gosec // test fixture paths
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != 1000 {
		t.Fatalf("expected 1000 lines, got %d", len(lines))
	}

	seen := make(map[string]bool, 1000)

	for _, line := range lines {
		if line == "" {
			t.Fatal("empty line in serialized log: writer interleaving bug")
		}

		if !strings.HasPrefix(line, "g") || !strings.Contains(line, "-l") {
			t.Fatalf("corrupt line: %q", line)
		}

		if seen[line] {
			t.Fatalf("duplicated line: %q", line)
		}

		seen[line] = true
	}
}
