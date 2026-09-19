package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stagedHardwareConfigRelPath mirrors the repo-relative location of the
// generated kexec hardware config, the file panix force-stages with
// intent-to-add because the testflakes flake imports it.
const stagedHardwareConfigRelPath = "tests/e2e/testflakes/hardware-configuration.nix"

// runTestGit runs git in dir and fails the test on error.
func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-C", dir}, args...)

	out, err := exec.CommandContext(t.Context(), "git", cmdArgs...).CombinedOutput() //nolint:gosec // git is fixed, args are test-controlled
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}

	return string(out)
}

// initIgnoredHardwareConfigRepo creates a git repository in dir with an initial
// commit that gitignores relPath, mirroring panix's committed .gitignore, then
// creates the ignored config file. The fixture commit (identity supplied
// inline) is required because git restore --staged reads HEAD; without one it
// cannot drop the intent-to-add entry.
func initIgnoredHardwareConfigRepo(t *testing.T, dir, relPath string) {
	t.Helper()

	runTestGit(t, dir, "-c", "init.defaultBranch=main", "init", "-q")

	err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(relPath+"\n"), filePerm)
	if err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	runTestGit(t, dir, "add", ".gitignore")
	runTestGit(t, dir, "-c", "user.name=panix-e2e", "-c", "user.email=panix-e2e@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture")

	absPath := filepath.Join(dir, relPath)

	err = os.MkdirAll(filepath.Dir(absPath), dirPerm)
	if err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}

	err = os.WriteFile(absPath, []byte("{}\n"), filePerm)
	if err != nil {
		t.Fatalf("write fixture config: %v", err)
	}
}

// configExists reports whether the generated config at path still exists.
func configExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func TestCleanupGeneratedHardwareConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	relPath := stagedHardwareConfigRelPath
	absPath := filepath.Join(dir, relPath)

	initIgnoredHardwareConfigRepo(t, dir, relPath)

	runTestGit(t, dir, "add", "-fN", "--", relPath)

	if !strings.Contains(runTestGit(t, dir, "ls-files"), relPath) {
		t.Fatal("expected an intent-to-add entry before cleanup")
	}

	cleanupGeneratedHardwareConfig(dir, absPath)

	if strings.Contains(runTestGit(t, dir, "ls-files"), relPath) {
		t.Error("intent-to-add entry still listed after cleanup")
	}

	if status := runTestGit(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("expected clean status after cleanup, got %q", status)
	}

	if configExists(absPath) {
		t.Error("generated config still present after cleanup")
	}

	// Idempotency: a second call with no index entry and no file is a no-op.
	cleanupGeneratedHardwareConfig(dir, absPath)

	if status := runTestGit(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("expected clean status after repeated cleanup, got %q", status)
	}

	if configExists(absPath) {
		t.Error("generated config still present after repeated cleanup")
	}
}
