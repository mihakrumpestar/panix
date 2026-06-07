package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/narqo/go-badge"
	"github.com/pkg/errors"
)

const (
	cacheDirName = ".cache"
	logDirName   = "log"
	sshKeyName   = "id_ed25519"
	dirPerm      = 0o750
	filePerm     = 0o600
	pubFilePerm  = 0o644
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

	_, err := exec.LookPath("genisoimage")
	if err != nil {
		_, err2 := exec.LookPath("mkisofs")
		if err2 != nil {
			return errors.New("neither genisoimage nor mkisofs found")
		}
	}

	return nil
}

func closeWithoutErrCheck(closer io.Closer) {
	_ = closer.Close()
}

func removeWithoutErrCheck(path string) {
	_ = os.RemoveAll(path)
}

func ensureSSHKeys() (string, error) {
	keyPath := filepath.Join(cacheDirPath, sshKeyName)

	_, err := os.Stat(keyPath)
	if err == nil {
		return keyPath, nil
	}

	err = exec.CommandContext(context.Background(), "ssh-keygen", //nolint:gosec
		"-t", "ed25519", "-N", "", "-f", keyPath).Run()
	if err != nil {
		return "", errors.Wrap(err, "ssh-keygen")
	}

	err = os.Chmod(keyPath, filePerm)
	if err != nil {
		return "", errors.Wrap(err, "chmod key")
	}

	return keyPath, nil
}

func downloadFile(downloadURL, dest string) error {
	err := os.MkdirAll(filepath.Dir(dest), dirPerm)
	if err != nil {
		return errors.Wrap(err, "create download dir")
	}

	file, err := os.Create(dest) //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "create download file")
	}

	defer closeWithoutErrCheck(file)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, downloadURL, nil)
	if err != nil {
		return errors.Wrapf(err, "download %s", downloadURL)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "download %s", downloadURL)
	}

	defer closeWithoutErrCheck(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("download %s: %s", downloadURL, resp.Status)
	}

	_, err = io.Copy(file, resp.Body)

	return errors.Wrapf(err, "write %s", dest)
}

func ensureCached(name, downloadURL string) error {
	path := filepath.Join(cacheDirPath, name)

	_, err := os.Stat(path)
	if err == nil {
		return nil
	}

	return downloadFile(downloadURL, path)
}

func stageSSHPubKey(keyPath string) error {
	destPath := filepath.Join(findProjectRoot(), "tests", "e2e", "testflakes", "ssh-key.pub")

	content, err := os.ReadFile(keyPath + ".pub") //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "read pub key")
	}

	return errors.Wrap(os.WriteFile(destPath, content, pubFilePerm), "stage pub key") //nolint:gosec
}

func buildInstallerISO() (string, error) {
	testflakesDir := filepath.Join(findProjectRoot(), "tests", "e2e", "testflakes")
	resultLink := filepath.Join(cacheDirPath, "installer-iso")

	out, err := exec.CommandContext(context.Background(), "nix", "build", //nolint:gosec
		"--option", "eval-cache", "false",
		"path:"+testflakesDir+"#installer-iso", "-o", resultLink).CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "build installer ISO: %s", string(out))
	}

	target, err := os.Readlink(resultLink)
	if err != nil {
		return "", errors.Wrap(err, "read iso symlink")
	}

	if !filepath.IsAbs(target) {
		target, err = filepath.Abs(filepath.Join(filepath.Dir(resultLink), target))
		if err != nil {
			return "", errors.Wrap(err, "resolve iso path")
		}
	}

	isoPath := findISOFile(target)
	if isoPath == "" {
		return "", errNoISO
	}

	return isoPath, nil
}

func buildKexecInstaller() (string, error) {
	testflakesDir := filepath.Join(findProjectRoot(), "tests", "e2e", "testflakes")

	cmd := exec.CommandContext(context.Background(), "nix", "build", //nolint:gosec
		"--option", "eval-cache", "false",
		"--print-out-paths",
		"path:"+testflakesDir+"#kexec-installer",
	)

	out, err := cmd.Output()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return "", errors.Wrapf(err, "build kexec installer: %s", string(exitErr.Stderr))
		}

		return "", errors.Wrap(err, "build kexec installer")
	}

	storePath := strings.TrimSpace(string(out))

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
	testflakesDir := filepath.Join(findProjectRoot(), "tests", "e2e", "testflakes")

	cmd := exec.CommandContext(context.Background(), "nix", "build", //nolint:gosec
		"--option", "eval-cache", "false",
		"--print-out-paths",
		"--no-link",
		"path:"+testflakesDir+"#"+flakeAttrPath,
	)

	out, err := cmd.Output()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return errors.Wrapf(err, "pre-build %s: %s", name, string(exitErr.Stderr))
		}

		return errors.Wrapf(err, "pre-build %s", name)
	}

	_ = strings.TrimSpace(string(out))

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
