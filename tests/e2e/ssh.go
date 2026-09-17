package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// readSystemProfileClosure resolves the store path the system profile points to.
func readSystemProfileClosure(keyPath string) (string, error) {
	output, err := sshRun(nixosISOPort, keyPath, "readlink -f /nix/var/nix/profiles/system")
	if err != nil {
		return "", errors.Wrap(err, "read system profile closure")
	}

	return strings.TrimSpace(output), nil
}

func readSystemProfileGeneration(keyPath string) (uint, error) {
	output, err := sshRun(nixosISOPort, keyPath,
		"nix-env --profile /nix/var/nix/profiles/system --list-generations")
	if err != nil {
		return 0, errors.Wrap(err, "list system generations")
	}

	generation, err := parseCurrentGeneration(output)
	if err != nil {
		return 0, err
	}

	return generation, nil
}

// parseCurrentGeneration reads the number from the "(current)" line.
func parseCurrentGeneration(output string) (uint, error) {
	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, "(current)") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			break
		}

		generation, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "parse generation number %q", fields[0])
		}

		return uint(generation), nil
	}

	return 0, errors.New("no current generation found")
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
		// First deploy legitimately lists no generations; by this phase the
		// deploy has run twice (bootstrap + deploy), so the listing must not be
		// empty.
		parGroup.Go("Verify home-manager generations listed (root, NixOS ISO VM)", func() error {
			return verifyHomeManagerGenerations(nixosISOPort, keyPath, "root")
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

// verifyHomeManagerMarkerAsUser checks the marker in the user's home by
// running cat as that user.
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

// verifyHomeManagerGenerations reproduces panix's own profile listing over
// SSH (reading panix's deployment log would be circular): the tilde must
// expand, and on this fixture nix-env's lock error names the expanded path,
// so a quoted tilde would surface as /root/~/... instead.
func verifyHomeManagerGenerations(port int, keyPath string, user string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	cmd := fmt.Sprintf(
		"su -l %s -c 'env NIX_PAGER=cat nix-env --profile ~/.local/state/nix/profiles/home-manager --list-generations 2>&1' || true",
		user,
	)

	output, err := sshRun(port, keyPath, cmd)
	if err != nil {
		return errors.Wrapf(err, "verify home-manager profile path on %s", addr)
	}

	if strings.TrimSpace(output) == "" {
		return errors.Errorf("empty output from tilde profile listing on %s (vacuous pass guard): %q", addr, output)
	}

	if strings.Contains(output, "/~/") || strings.Contains(output, `"~/`) {
		return errors.Errorf("tilde profile path not expanded on %s (quoted tilde regression): %q", addr, output)
	}

	return nil
}

// verifyKexecBootstrapArtifacts pins the bootstrap fixtures: the kexec hook
// marker and both hardware configs written locally, not on the targets. The
// test-vm-kexec import is the ordering proof: without the post-kexec
// generation the bootstrap disko build fails before this verification runs.
func verifyKexecBootstrapArtifacts(kexecPort, isoPort int, keyPath string) error {
	parGroup := newParallelGroup()

	parGroup.Go("Verify shell-string bootstrap hook on kexec VM", func() error {
		return verifyBootstrapHook(kexecPort, keyPath)
	})

	parGroup.Go("Verify hardware config generation ran on both VMs", verifyHardwareConfigGenerationLog)

	parGroup.Go("Verify ISO hardware config written locally, not on the target", func() error {
		return verifyHardwareConfigLocalWrite(isoPort, keyPath)
	})

	parGroup.Go("Verify kexec hardware config written locally, not on the target", func() error {
		return verifyKexecHardwareConfigLocalWrite(kexecPort, keyPath)
	})

	return parGroup.Wait()
}

func verifyBootstrapHook(kexecPort int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", kexecPort)

	output, err := sshRun(kexecPort, keyPath, "test -e /tmp/e2e-hook-ran && echo hook-ok || echo hook-missing")
	if err != nil {
		return errors.Wrapf(err, "verify bootstrap hook on %s", addr)
	}

	if !strings.Contains(output, "hook-ok") {
		return errors.Errorf("shell-string bootstrap hook did not run on %s: %q", addr, output)
	}

	return nil
}

// verifyHardwareConfigGenerationLog reads the bootstrap log as the execution
// evidence on the targets: the generated configs themselves are local files.
func verifyHardwareConfigGenerationLog() error {
	matches, _ := filepath.Glob(filepath.Join(logDirPath, "panix-bootstrap.*.log"))
	if len(matches) == 0 {
		return errors.Errorf("no bootstrap log found in %s", logDirPath)
	}

	logPath := matches[len(matches)-1]

	content, err := os.ReadFile(logPath) //nolint:gosec // repo-local test log
	if err != nil {
		return errors.Wrapf(err, "read bootstrap log %s", logPath)
	}

	log := string(content)

	count := strings.Count(log, `"description":"generate config"`)
	if count < 2 {
		return errors.Errorf("expected hardware config generation for the ISO and kexec VMs in %s, found %d", logPath, count)
	}

	if !strings.Contains(log, "nixos-generate-config --show-hardware-config --no-filesystems") {
		return errors.Errorf("nixos-generate-config command not found in %s", logPath)
	}

	return nil
}

// verifyHardwareConfigLocalWrite checks the ISO config is written locally, not
// on the target.
func verifyHardwareConfigLocalWrite(isoPort int, keyPath string) error {
	err := verifyGeneratedHardwareConfig(e2eHardwareConfigPath)
	if err != nil {
		return err
	}

	return verifyHardwareConfigAbsentOnTarget(isoPort, keyPath, e2eHardwareConfigPath)
}

// verifyKexecHardwareConfigLocalWrite checks the kexec config landed inside the
// testflakes tree (the path test-vm-kexec imports), not on the target.
func verifyKexecHardwareConfigLocalWrite(kexecPort int, keyPath string) error {
	err := verifyGeneratedHardwareConfig(e2eKexecHardwareConfigPath)
	if err != nil {
		return err
	}

	return verifyHardwareConfigAbsentOnTarget(kexecPort, keyPath, e2eKexecHardwareConfigPath)
}

// verifyGeneratedHardwareConfig asserts the locally written file looks like
// nixos-generate-config output.
func verifyGeneratedHardwareConfig(path string) error {
	content, err := os.ReadFile(path) //nolint:gosec // repo-local test path
	if err != nil {
		return errors.Wrapf(err, "hardware config must be written locally to %s", path)
	}

	trimmed := strings.TrimSpace(string(content))
	if !strings.Contains(trimmed, "modulesPath") || !strings.Contains(trimmed, "config,") ||
		!strings.HasSuffix(trimmed, "}") {
		return errors.Errorf("local hardware config %s is not plausible nixos-generate-config output: %q", path, trimmed)
	}

	return nil
}

func verifyHardwareConfigAbsentOnTarget(port int, keyPath, path string) error {
	targetAddr := fmt.Sprintf("127.0.0.1:%d", port)

	output, err := sshRun(port, keyPath, "test -e '"+path+"' && echo target-has-config || echo target-clean")
	if err != nil {
		return errors.Wrapf(err, "verify hardware config absence on %s", targetAddr)
	}

	if !strings.Contains(output, "target-clean") {
		return errors.Errorf("hardware config must not be written on the target %s: %q", targetAddr, output)
	}

	return nil
}

func verifyPackage(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Login shell so /etc/profile.d/nix.sh is sourced on Debian (plain SSH
	// sessions do not source it, so nix-profile binaries are not in PATH);
	// NixOS handles this via PAM, the login shell works there too.
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

// verifyMaidPackage checks the maid activation script ran on the given VM: it
// creates ~/.panix-maid-test-marker as a symlink into the nix store, and cat
// follows the symlink to the marker text.
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

	// || true after hello: a non-zero exit (hello not installed) would make
	// sshRun fail and mask which check actually failed.
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
