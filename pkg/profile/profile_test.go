package profile

import (
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMkdirForFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "profile.out")

	require.NoError(t, mkdirForFile(path))

	info, statErr := os.Stat(filepath.Join(tmpDir, "subdir", "nested"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestMkdirForFileExistingDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	require.NoError(t, mkdirForFile(filepath.Join(tmpDir, "profile.out")))
}

func TestStartEmpty(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	stop, err := Start(Profile{})
	require.NoError(t, err)
	require.NotNil(t, stop)
	stop()
}

func TestStartCPU(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	cpuPath := filepath.Join(tmpDir, "cpu.prof")

	stop, err := Start(Profile{CPU: cpuPath})
	require.NoError(t, err)
	stop()

	info, statErr := os.Stat(cpuPath)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Size())
}

func TestStartMem(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, "mem.prof")

	stop, err := Start(Profile{Mem: memPath})
	require.NoError(t, err)
	stop()

	info, statErr := os.Stat(memPath)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Size())
}

func TestStartGoroutine(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	goroutinePath := filepath.Join(tmpDir, "goroutine.prof")

	stop, err := Start(Profile{Goroutine: goroutinePath})
	require.NoError(t, err)
	stop()

	info, statErr := os.Stat(goroutinePath)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Size())
}

func TestStartBlock(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	blockPath := filepath.Join(tmpDir, "block.prof")

	stop, err := Start(Profile{Block: blockPath})
	require.NoError(t, err)
	stop()

	_, statErr := os.Stat(blockPath)
	require.NoError(t, statErr)
}

func TestStartMutex(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	mutexPath := filepath.Join(tmpDir, "mutex.prof")

	stop, err := Start(Profile{Mutex: mutexPath})
	require.NoError(t, err)
	stop()

	_, statErr := os.Stat(mutexPath)
	require.NoError(t, statErr)
}

func TestStartAllProfiles(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()

	stop, err := Start(Profile{
		CPU:       filepath.Join(tmpDir, "cpu.prof"),
		Mem:       filepath.Join(tmpDir, "mem.prof"),
		Goroutine: filepath.Join(tmpDir, "goroutine.prof"),
	})
	require.NoError(t, err)
	stop()

	for _, name := range []string{"cpu.prof", "mem.prof", "goroutine.prof"} {
		path := filepath.Join(tmpDir, name)

		info, statErr := os.Stat(path)
		require.NoError(t, statErr, "stat %s", name)
		assert.NotZero(t, info.Size(), "%s is empty", name)
	}
}

func TestStartCPUInvalidPath(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	_, err := Start(Profile{CPU: "/dev/null/cpu.prof"})
	assert.Error(t, err)
}

func TestWriteProfile(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "goroutine.prof")

	require.NoError(t, writeProfile("goroutine", path))

	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Size())
}

func TestWriteProfileInvalidName(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.prof")

	assert.Error(t, writeProfile("nonexistent_profile_xyz", path))
}

func TestWriteProfileInvalidPath(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	assert.Error(t, writeProfile("goroutine", "/dev/null/goroutine.prof"))
}

func TestWriteProfileCreatesDir(t *testing.T) { //nolint:paralleltest // manipulates global pprof state
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "dir", "goroutine.prof")

	require.NoError(t, writeProfile("goroutine", path))

	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Size())
}

func TestDirPermissions(t *testing.T) {
	t.Parallel()

	assert.Equal(t, dirPermissions, os.FileMode(0750))
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

	assert.Equal(t, "cpu.prof", prof.CPU)
	assert.Equal(t, "mem.prof", prof.Mem)
}

func TestPprofLookupGoroutine(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, pprof.Lookup("goroutine"))
}

func TestPprofLookupBlock(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, pprof.Lookup("block"))
}

func TestPprofLookupMutex(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, pprof.Lookup("mutex"))
}
