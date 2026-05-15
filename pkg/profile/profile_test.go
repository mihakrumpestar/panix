package profile

import (
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
)

func TestMkdirForFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "profile.out")

	err := mkdirForFile(path)
	if err != nil {
		t.Fatalf("mkdirForFile() error: %v", err)
	}

	info, statErr := os.Stat(filepath.Join(tmpDir, "subdir", "nested"))
	if statErr != nil {
		t.Fatalf("stat nested dir error: %v", statErr)
	}

	if !info.IsDir() {
		t.Error("expected directory to exist")
	}
}

func TestMkdirForFileExistingDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	err := mkdirForFile(filepath.Join(tmpDir, "profile.out"))
	if err != nil {
		t.Fatalf("mkdirForFile() on existing dir error: %v", err)
	}
}

func TestStartEmpty(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	stop, err := Start(Profile{})
	if err != nil {
		t.Fatalf("Start() with empty profile error: %v", err)
	}

	if stop == nil {
		t.Fatal("Start() returned nil stop func")
	}

	stop()
}

func TestStartCPU(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	cpuPath := filepath.Join(tmpDir, "cpu.prof")

	stop, err := Start(Profile{CPU: cpuPath})
	if err != nil {
		t.Fatalf("Start() with CPU profile error: %v", err)
	}

	stop()

	info, statErr := os.Stat(cpuPath)
	if statErr != nil {
		t.Fatalf("stat CPU profile error: %v", statErr)
	}

	if info.Size() == 0 {
		t.Error("CPU profile file is empty")
	}
}

func TestStartMem(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, "mem.prof")

	stop, err := Start(Profile{Mem: memPath})
	if err != nil {
		t.Fatalf("Start() with Mem profile error: %v", err)
	}

	stop()

	info, statErr := os.Stat(memPath)
	if statErr != nil {
		t.Fatalf("stat Mem profile error: %v", statErr)
	}

	if info.Size() == 0 {
		t.Error("Mem profile file is empty")
	}
}

func TestStartGoroutine(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	goroutinePath := filepath.Join(tmpDir, "goroutine.prof")

	stop, err := Start(Profile{Goroutine: goroutinePath})
	if err != nil {
		t.Fatalf("Start() with Goroutine profile error: %v", err)
	}

	stop()

	info, statErr := os.Stat(goroutinePath)
	if statErr != nil {
		t.Fatalf("stat Goroutine profile error: %v", statErr)
	}

	if info.Size() == 0 {
		t.Error("Goroutine profile file is empty")
	}
}

func TestStartBlock(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	blockPath := filepath.Join(tmpDir, "block.prof")

	stop, err := Start(Profile{Block: blockPath})
	if err != nil {
		t.Fatalf("Start() with Block profile error: %v", err)
	}

	stop()

	_, statErr := os.Stat(blockPath)
	if statErr != nil {
		t.Fatalf("stat Block profile error: %v", statErr)
	}
}

func TestStartMutex(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	mutexPath := filepath.Join(tmpDir, "mutex.prof")

	stop, err := Start(Profile{Mutex: mutexPath})
	if err != nil {
		t.Fatalf("Start() with Mutex profile error: %v", err)
	}

	stop()

	_, statErr := os.Stat(mutexPath)
	if statErr != nil {
		t.Fatalf("stat Mutex profile error: %v", statErr)
	}
}

func TestStartAllProfiles(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()

	stop, err := Start(Profile{
		CPU:       filepath.Join(tmpDir, "cpu.prof"),
		Mem:       filepath.Join(tmpDir, "mem.prof"),
		Goroutine: filepath.Join(tmpDir, "goroutine.prof"),
	})
	if err != nil {
		t.Fatalf("Start() with multiple profiles error: %v", err)
	}

	stop()

	for _, name := range []string{"cpu.prof", "mem.prof", "goroutine.prof"} {
		path := filepath.Join(tmpDir, name)

		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("stat %s error: %v", name, statErr)

			continue
		}

		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestStartCPUInvalidPath(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	_, err := Start(Profile{CPU: "/nonexistent/deep/nested/dir/cpu.prof"})
	if err == nil {
		t.Error("Start() with invalid CPU path should return error")
	}
}

func TestWriteProfile(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "goroutine.prof")

	err := writeProfile("goroutine", path)
	if err != nil {
		t.Fatalf("writeProfile() error: %v", err)
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat error: %v", statErr)
	}

	if info.Size() == 0 {
		t.Error("profile file is empty")
	}
}

func TestWriteProfileInvalidName(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.prof")

	err := writeProfile("nonexistent_profile_xyz", path)
	if err == nil {
		t.Error("writeProfile() with invalid name should return error")
	}
}

func TestWriteProfileInvalidPath(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	err := writeProfile("goroutine", "/nonexistent/deep/nested/dir/goroutine.prof")
	if err == nil {
		t.Error("writeProfile() with invalid path should return error")
	}
}

func TestWriteProfileCreatesDir(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "dir", "goroutine.prof")

	err := writeProfile("goroutine", path)
	if err != nil {
		t.Fatalf("writeProfile() error: %v", err)
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat error: %v", statErr)
	}

	if info.Size() == 0 {
		t.Error("profile file is empty")
	}
}

func TestDirPermissions(t *testing.T) {
	t.Parallel()

	if dirPermissions != os.FileMode(0750) {
		t.Errorf("dirPermissions = %o, want 0750", dirPermissions)
	}
}

func TestProfileFieldTags(t *testing.T) {
	t.Parallel()

	prof := Profile{
		CPU:       "cpu.prof",
		Mem:       "mem.prof",
		Block:     "block.prof",
		Mutex:     "mutex.prof",
		Goroutine: "goroutine.prof",
	}

	if prof.CPU != "cpu.prof" {
		t.Errorf("CPU = %q, want %q", prof.CPU, "cpu.prof")
	}

	if prof.Mem != "mem.prof" {
		t.Errorf("Mem = %q, want %q", prof.Mem, "mem.prof")
	}
}

func TestPprofLookupGoroutine(t *testing.T) {
	t.Parallel()

	prof := pprof.Lookup("goroutine")
	if prof == nil {
		t.Error("pprof.Lookup(goroutine) returned nil")
	}
}

func TestPprofLookupBlock(t *testing.T) {
	t.Parallel()

	prof := pprof.Lookup("block")
	if prof == nil {
		t.Error("pprof.Lookup(block) returned nil")
	}
}

func TestPprofLookupMutex(t *testing.T) {
	t.Parallel()

	prof := pprof.Lookup("mutex")
	if prof == nil {
		t.Error("pprof.Lookup(mutex) returned nil")
	}
}
