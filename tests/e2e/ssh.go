package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const (
	sshWaitTimeout           = 2 * time.Minute
	sshPollInterval          = 500 * time.Millisecond
	sshHandshakeTimeout      = 5 * time.Second
	sshRunTimeout            = 30 * time.Second
	splitParts               = 2
	systemManagerVerifyParts = 3
	markerContent            = "panix-e2e-test-pass"
)

var errSSHTimeout = errors.New("SSH wait timed out")

func sshConfig(keyPath string) (*ssh.ClientConfig, error) {
	keyContent, err := os.ReadFile(keyPath) //nolint:gosec
	if err != nil {
		return nil, errors.Wrap(err, "read SSH key")
	}

	signer, err := ssh.ParsePrivateKey(keyContent)
	if err != nil {
		return nil, errors.Wrap(err, "parse SSH key")
	}

	return &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // intentional for e2e tests
		Timeout:         sshHandshakeTimeout,
	}, nil
}

func waitForSSH(port int, keyPath string) error {
	config, err := sshConfig(keyPath)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(sshWaitTimeout)

	for time.Now().Before(deadline) {
		conn, dialErr := ssh.Dial("tcp", addr, config)
		if dialErr == nil {
			closeWithoutErrCheck(conn)

			return nil
		}

		time.Sleep(sshPollInterval)
	}

	return errors.Wrapf(errSSHTimeout, "SSH not available at %s after %v", addr, sshWaitTimeout)
}

func sshRun(port int, keyPath string, command string) (string, error) {
	config, err := sshConfig(keyPath)
	if err != nil {
		return "", err
	}

	config.Timeout = sshRunTimeout

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", errors.Wrapf(err, "SSH dial %s", addr)
	}

	defer closeWithoutErrCheck(conn)

	session, err := conn.NewSession()
	if err != nil {
		return "", errors.Wrap(err, "SSH session")
	}

	defer closeWithoutErrCheck(session)

	var stdout bytes.Buffer

	session.Stdout = &stdout

	err = session.Run(command)

	return stdout.String(), errors.Wrap(err, "SSH run")
}

func verifyNixOSInstallation(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	output, err := sshRun(port, keyPath, "cat /etc/panix-test-marker; echo '---'; cat /etc/os-release")
	if err != nil {
		return errors.Wrapf(err, "verify SSH on %s", addr)
	}

	parts := strings.SplitN(output, "---", splitParts)
	if len(parts) != splitParts {
		return errors.Errorf("unexpected verify output on %s", addr)
	}

	marker := strings.TrimSpace(parts[0])
	if !strings.Contains(marker, markerContent) {
		return errors.Errorf("marker file mismatch on %s: %s", addr, marker)
	}

	osRelease := parts[1]
	if !strings.Contains(osRelease, "ID=nixos") {
		return errors.Errorf("expected NixOS on %s", addr)
	}

	if strings.Contains(osRelease, "VARIANT_ID=installer") {
		return errors.Errorf("still installer on %s", addr)
	}

	return nil
}

func verifyHomeManager(keyPath string) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		parGroup.Go("Verify home-manager (root) on NixOS ISO VM", func() error {
			return verifyHomeManagerMarker(nixosISOPort, keyPath, "root")
		})
		parGroup.Go("Verify home-manager (root) on Debian-nix VM", func() error {
			return verifyHomeManagerMarker(debianNixVMPort, keyPath, "root")
		})
		parGroup.Go("Verify home-manager (alice) on NixOS ISO VM", func() error {
			return verifyHomeManagerMarkerAsUser(nixosISOPort, keyPath, "alice")
		})
		parGroup.Go("Verify home-manager (alice) on Debian-nix VM", func() error {
			return verifyHomeManagerMarkerAsUser(debianNixVMPort, keyPath, "alice")
		})
	}

	return parGroup.Wait()
}

func verifyHomeManagerMarker(port int, keyPath string, user string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	path := fmt.Sprintf("/%s/.panix-home-test-marker", user)

	output, err := sshRun(port, keyPath, "cat "+path)
	if err != nil {
		return errors.Wrapf(err, "verify home-manager on %s", addr)
	}

	marker := strings.TrimSpace(output)
	if !strings.Contains(marker, markerContent) {
		return errors.Errorf("home-manager marker not found on %s: %s", addr, marker)
	}

	return nil
}

// verifyHomeManagerMarkerAsUser checks the marker file exists in the user's
// home directory by running `cat` as that user via sudo.
func verifyHomeManagerMarkerAsUser(port int, keyPath string, user string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	cmd := fmt.Sprintf("su -l %s -c 'cat ~/.panix-home-test-marker'", user)

	output, err := sshRun(port, keyPath, cmd)
	if err != nil {
		return errors.Wrapf(err, "verify home-manager (user=%s) on %s", user, addr)
	}

	marker := strings.TrimSpace(output)
	if !strings.Contains(marker, markerContent) {
		return errors.Errorf("home-manager marker not found for user %s on %s: %s", user, addr, marker)
	}

	return nil
}

func verifyPackage(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Use a login shell so that /etc/profile.d/nix.sh is sourced on Debian
	// (non-interactive SSH sessions don't source it, so nix-profile binaries
	// aren't in PATH). On NixOS PAM handles this for all sessions, but the
	// login shell works there too.
	output, err := sshRun(port, keyPath, "su -l root -c 'panix-package-marker'")
	if err != nil {
		return errors.Wrapf(err, "verify package on %s", addr)
	}

	marker := strings.TrimSpace(output)
	if !strings.Contains(marker, markerContent) {
		return errors.Errorf("package marker not found on %s: %s", addr, marker)
	}

	return nil
}

func verifyPackages(keyPath string) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		parGroup.Go("Verify package on NixOS ISO VM", func() error {
			return verifyPackage(nixosISOPort, keyPath)
		})
		parGroup.Go("Verify package on Debian-nix VM", func() error {
			return verifyPackage(debianNixVMPort, keyPath)
		})
	}

	return parGroup.Wait()
}

// verifyMaidPackage checks that the maid activation script ran on the given
// VM. The real nix-maid bundle's activate script (run as root, via
// `su -l root -c`) creates ~/.panix-maid-test-marker as a symlink into the
// nix store via systemd-tmpfiles; `cat` follows the symlink, so verification
// reads it back and checks the marker text.
func verifyMaidPackage(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	output, err := sshRun(port, keyPath, "cat /root/.panix-maid-test-marker")
	if err != nil {
		return errors.Wrapf(err, "verify maid package on %s", addr)
	}

	marker := strings.TrimSpace(output)
	if !strings.Contains(marker, markerContent) {
		return errors.Errorf("maid package marker not found on %s: %s", addr, marker)
	}

	return nil
}

func verifyMaidPackages(keyPath string) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		parGroup.Go("Verify maid package on NixOS ISO VM", func() error {
			return verifyMaidPackage(nixosISOPort, keyPath)
		})
		parGroup.Go("Verify maid package on Debian-nix VM", func() error {
			return verifyMaidPackage(debianNixVMPort, keyPath)
		})
	}

	return parGroup.Wait()
}

func verifySystemManager(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Use || true after hello so that a non-zero exit (e.g. hello not installed)
	// doesn't cause sshRun to return an error that masks which check failed.
	output, err := sshRun(port, keyPath,
		"cat /etc/panix-test-marker; echo '---'; "+
			"cat /etc/os-release; echo '---'; "+
			"/run/system-manager/sw/bin/hello --version || true")
	if err != nil {
		return errors.Wrapf(err, "verify system-manager on %s", addr)
	}

	parts := strings.SplitN(output, "---", systemManagerVerifyParts)
	if len(parts) < systemManagerVerifyParts {
		return errors.Errorf("unexpected verify output on %s: expected 3 sections, got %d", addr, len(parts))
	}

	marker := strings.TrimSpace(parts[0])
	if !strings.Contains(marker, markerContent) {
		return errors.Errorf("system-manager marker not found on %s: %s", addr, marker)
	}

	osRelease := parts[1]
	if !strings.Contains(osRelease, "ID=debian") {
		return errors.Errorf("expected Debian on %s, got: %s", addr, osRelease)
	}

	helloVersion := strings.TrimSpace(parts[2])
	if !strings.Contains(helloVersion, "hello") {
		return errors.Errorf("hello package not found via system-manager on %s: %s", addr, helloVersion)
	}

	return nil
}

func verifySystemManagers(keyPath string) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		parGroup.Go("Verify system-manager on Debian-nix VM", func() error {
			return verifySystemManager(debianNixVMPort, keyPath)
		})
	}

	return parGroup.Wait()
}
