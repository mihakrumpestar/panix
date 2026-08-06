package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/narqo/go-badge"
	"github.com/pkg/errors"
)

const (
	cacheDirName       = ".cache"
	logDirName         = "log"
	dirPerm            = 0o750
	filePerm           = 0o600
	pubFilePerm        = 0o644
	requiredSSHKeyPerm = 0o600
)

var errKVM = errors.New("KVM not available (/dev/kvm missing)")

var errNoISO = errors.New("no .iso found in build output")

var (
	cacheDirPath string
	logDirPath   string
)

func initDirs() error {
	root := findProjectRoot()
	cacheDirPath = filepath.Join(root, "tests", "e2e", cacheDirName)
	logDirPath = filepath.Join(root, "tests", "e2e", logDirName)

	err := os.MkdirAll(cacheDirPath, dirPerm)
	if err != nil {
		return errors.Wrap(err, "create cache dir")
	}

	_ = os.RemoveAll(logDirPath)

	return errors.Wrap(os.MkdirAll(logDirPath, dirPerm), "create log dir")
}

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return dir
}

func checkKVM() error {
	_, err := os.Stat("/dev/kvm")
	if err != nil {
		return errKVM
	}

	return nil
}

func checkDeps() error {
	for _, dep := range []string{"qemu-system-x86_64", "qemu-img", "nix", "curl"} {
		_, err := exec.LookPath(dep)
		if err != nil {
			return errors.Errorf("dependency %q not found", dep)
		}
	}

	return nil
}

func closeWithoutErrCheck(closer io.Closer) {
	_ = closer.Close()
}

// ensureSSHKeys returns the path to the committed test-only SSH key pair in
// testflakes/ssh.key. The key pair is static and committed to the repo so that
// nix builds (which read ssh.pub via builtins.readFile) work without a
// prior test run.
func ensureSSHKeys() (string, error) {
	keyPath := filepath.Join(findProjectRoot(), "tests", "e2e", "testflakes", "ssh.key")

	info, err := os.Stat(keyPath)
	if err != nil {
		return "", errors.Wrap(err, "test SSH key not found in testflakes/ssh.key — ensure the key pair is committed")
	}

	// SSH refuses to use private keys that are readable by others (exit status 255,
	// "bad permissions"). The Go SSH library (used by waitForSSH) doesn't enforce
	// this, so the reachability check passes but panix's SSH command fails.
	// Enforce 0600 here so the failure is caught early with a clear message.
	if info.Mode().Perm() != requiredSSHKeyPerm {
		return "", errors.Errorf("test SSH key %s has permissions %o, expected 0600 — run: chmod 600 %s", keyPath, info.Mode().Perm(), keyPath)
	}

	return keyPath, nil
}

// nixBuild builds a flake attribute from testflakes and returns the store path.
func nixBuild(attr string) (string, error) {
	testflakesDir := filepath.Join(findProjectRoot(), "tests", "e2e", "testflakes")

	out, err := exec.CommandContext(context.Background(), "nix", "build", //nolint:gosec
		"--print-out-paths", "--no-link",
		"path:"+testflakesDir+"#"+attr,
	).Output()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return "", errors.Wrapf(err, "nix build %s: %s", attr, string(exitErr.Stderr))
		}

		return "", errors.Wrapf(err, "nix build %s", attr)
	}

	return strings.TrimSpace(string(out)), nil
}

func buildInstallerISO() (string, error) {
	storePath, err := nixBuild("installer-iso")
	if err != nil {
		return "", err
	}

	isoPath := findISOFile(storePath)
	if isoPath == "" {
		return "", errNoISO
	}

	return isoPath, nil
}

func buildKexecInstaller() (string, error) {
	storePath, err := nixBuild("kexec-installer")
	if err != nil {
		return "", err
	}

	kexecPath := findTarballFile(storePath)
	if kexecPath == "" {
		return "", errors.Errorf("no tarball found in kexec build output: %s", storePath)
	}

	return kexecPath, nil
}

func findTarballFile(storePath string) string {
	// kexecInstallerTarball places .tar.gz at the store root
	entries, err := os.ReadDir(storePath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tar.gz") {
				return filepath.Join(storePath, entry.Name())
			}
		}
	}

	// kexecTarball (new format) places .tar.xz in tarball/ subdirectory
	candidate := filepath.Join(storePath, "tarball")

	entries, err = os.ReadDir(candidate)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".tar.xz") || strings.HasSuffix(entry.Name(), ".tar.gz")) {
				return filepath.Join(candidate, entry.Name())
			}
		}
	}

	return ""
}

const (
	cacheServerWaitTimeout = 10 * time.Second
	cacheServerPollDelay   = 20 * time.Millisecond
)

func startNixCacheServer() (*exec.Cmd, error) {
	cfgPath := filepath.Join(findProjectRoot(), "tests", "e2e", "harmonia.toml")

	cmd := exec.CommandContext(context.Background(), "harmonia-cache")

	cmd.Env = append(os.Environ(), "CONFIG_FILE="+cfgPath)

	err := cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "start nix cache server")
	}

	dialer := &net.Dialer{Timeout: time.Second}

	deadline := time.Now().Add(cacheServerWaitTimeout)
	for time.Now().Before(deadline) {
		conn, dialErr := dialer.DialContext(context.Background(), "tcp", "127.0.0.1:5000")
		if dialErr == nil {
			closeWithoutErrCheck(conn)

			return cmd, nil
		}

		time.Sleep(cacheServerPollDelay)
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	return nil, errors.New("nix cache server did not start within 10s")
}

func preBuildClosure(name, flakeAttrPath string) error {
	_, err := nixBuild(flakeAttrPath)
	if err != nil {
		return errors.Wrapf(err, "pre-build %s", name)
	}

	return nil
}

func findISOFile(storePath string) string {
	candidate := filepath.Join(storePath, "iso")

	entries, err := os.ReadDir(candidate)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".iso") {
				return filepath.Join(candidate, entry.Name())
			}
		}
	}

	info, statErr := os.Stat(storePath)
	if statErr == nil && !info.IsDir() {
		return storePath
	}

	return ""
}

func writeBadgeSVG(passed bool) error {
	genDir := filepath.Join(findProjectRoot(), "gen")

	err := os.MkdirAll(genDir, dirPerm)
	if err != nil {
		return errors.Wrap(err, "create gen dir")
	}

	status := "passing"
	color := badge.Color("#4c1")

	if !passed {
		status = "failing"
		color = badge.Color("#e05d44")
	}

	msg := fmt.Sprintf("%s | %s", status, time.Now().Format("2006-01-02"))

	file, err := os.Create(filepath.Join(genDir, "e2e.svg")) //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "create badge svg")
	}

	defer closeWithoutErrCheck(file)

	return errors.Wrap(badge.Render("e2e", msg, color, file), "render badge")
}
