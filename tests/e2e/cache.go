package main

import (
	"context"
	"fmt"
	"io"
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

	target, readErr := os.Readlink(resultLink)
	if readErr == nil {
		if isoPath := findISOFile(target); isoPath != "" {
			return isoPath, nil
		}
	}

	out, err := exec.CommandContext(context.Background(), "nix", "build", //nolint:gosec
		"path:"+testflakesDir+"#installer-iso", "-o", resultLink).CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "build installer ISO: %s", string(out))
	}

	target, err = os.Readlink(resultLink)
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
