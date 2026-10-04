package main

import (
	"bytes"
	"encoding/json"
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

const (
	hardwareConfigLogDescription = "generate config"
	hardwareConfigLogSuccess     = "success"

	isoVMHardwareConfigXPath   = "test/nixosConfigurations/test-vm/nixos-iso-vm"
	kexecVMHardwareConfigXPath = "test/nixosConfigurations/test-vm-kexec/kexec-vm"

	nixosGenerateConfigArg     = "nixos-generate-config"
	nixosShowHardwareConfigArg = "--show-hardware-config"
	nixosNoFilesystemsArg      = "--no-filesystems"
)

// hardwareConfigLogEntry is the subset of executioner JSON log fields needed to
// prove hardware config generation. Entries without a matching description or
// status are ignored by the verifier.
type hardwareConfigLogEntry struct {
	XPath       string `json:"xpath"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Command     string `json:"command"`
}

// requiredHardwareConfigXPaths returns every xpath that must show a successful
// hardware config generation in the bootstrap log.
func requiredHardwareConfigXPaths() []string {
	return []string{isoVMHardwareConfigXPath, kexecVMHardwareConfigXPath}
}

// successfulHardwareConfigEntries maps xpath to the successful "generate
// config" JSON entries found in the log. Non-JSON lines and entries with a
// different description or status are ignored.
func successfulHardwareConfigEntries(log string) map[string]hardwareConfigLogEntry {
	entries := make(map[string]hardwareConfigLogEntry)

	for line := range strings.SplitSeq(log, "\n") {
		if !strings.Contains(line, hardwareConfigLogDescription) {
			continue
		}

		var entry hardwareConfigLogEntry

		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			continue
		}

		if entry.Description != hardwareConfigLogDescription || entry.Status != hardwareConfigLogSuccess {
			continue
		}

		entries[entry.XPath] = entry
	}

	return entries
}

// verifyHardwareConfigGenerationLogContent proves the bootstrap log records a
// successful hardware config generation for every required xpath and that each
// logged command carries every required argv token. JSON parsing plus
// token-by-token command checks keep the assertion quoting-agnostic: the
// executioner shell-quotes each argv element, so a raw substring match on the
// joined command breaks as soon as quoting is introduced.
func verifyHardwareConfigGenerationLogContent(log string) error {
	requiredXPaths := requiredHardwareConfigXPaths()
	successful := successfulHardwareConfigEntries(log)

	var found, missing []string

	for _, xpath := range requiredXPaths {
		_, ok := successful[xpath]
		if ok {
			found = append(found, xpath)
		} else {
			missing = append(missing, xpath)
		}
	}

	if len(missing) > 0 {
		return errors.Errorf(
			"hardware config generation not proven: successful xpaths %q, missing %q",
			found, missing,
		)
	}

	return verifyHardwareConfigCommandArgs(successful)
}

// verifyHardwareConfigCommandArgs asserts every required argv token appears in
// the logged command for each xpath, independently of shell quoting.
func verifyHardwareConfigCommandArgs(successful map[string]hardwareConfigLogEntry) error {
	requiredArgs := []string{nixosGenerateConfigArg, nixosShowHardwareConfigArg, nixosNoFilesystemsArg}

	for _, xpath := range requiredHardwareConfigXPaths() {
		entry := successful[xpath]

		for _, arg := range requiredArgs {
			if !strings.Contains(entry.Command, arg) {
				return errors.Errorf(
					"hardware config generation for xpath %q: logged command missing token %q: %q",
					xpath, arg, entry.Command,
				)
			}
		}
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

	return errors.Wrapf(verifyHardwareConfigGenerationLogContent(string(content)), "verify bootstrap log %s", logPath)
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

// Nix-install bootstrap probes and their marker words. Each probe echoes
// exactly one marker of its pair; the matching verifier asserts the ok/present
// marker so the command string and the check share one naming source.
const (
	nixBinPresentMarker    = "nixbin-present"
	nixBinAbsentMarker     = "nixbin-absent"
	nixDirPresentMarker    = "nixdir-present"
	nixDirAbsentMarker     = "nixdir-absent"
	nixBinOKMarker         = "nixbin-ok"
	nixBinMissingMarker    = "nixbin-missing"
	nixStorePathPrefix     = "/nix/store/"
	nixProbeDirSection     = "test -e /nix && echo " + nixDirPresentMarker + " || echo " + nixDirAbsentMarker
	nixProbeBinSection     = "command -v nix >/dev/null 2>&1 && echo " + nixBinPresentMarker + " || echo " + nixBinAbsentMarker
	nixProbeVersionSection = "nix --version >/dev/null 2>&1 && echo " + nixBinOKMarker + " || echo " + nixBinMissingMarker
)

// Nix-install secret landing probe: the OS secret must land on the live root
// at the plain remote path with the exact fixture content, and must not exist
// under the bootstrapping root (the redirect belongs to the NixOS bootstrap
// only). Same marker-pair style as the nix probes: the probe echoes exactly
// one marker of its pair so the command string and the check share one naming
// source.
const (
	// nixinstallSecretRemotePath mirrors the secret remote_path in panix.yml.
	nixinstallSecretRemotePath = "/etc/panix-e2e-secret-nixinstall" //nolint:gosec // remote path name, not a credential
	// nixinstallSecretFixtureName is the committed fixture panix.yml
	// transfers; readSecretFixture is the single reader of its bytes.
	nixinstallSecretFixtureName = "secret-nixinstall-fixture.txt"
	// bootstrappingRootPath is the root Machine.MaybeBootstrappingPath
	// redirects to while a NixOS bootstrap is in progress.
	bootstrappingRootPath = "/mnt"

	secretLandingPlainMissingMarker = "secret-plain-missing"
	secretLandingMntPresentMarker   = "secret-mnt-present" //nolint:gosec // marker name, not a credential
	secretLandingMntAbsentMarker    = "secret-mnt-absent"  //nolint:gosec // marker name, not a credential

	secretLandingPlainProbeSection = "cat -- " + nixinstallSecretRemotePath +
		" 2>/dev/null || echo " + secretLandingPlainMissingMarker
	secretLandingMntProbeSection = "test -e " + bootstrappingRootPath + nixinstallSecretRemotePath +
		" && echo " + secretLandingMntPresentMarker + " || echo " + secretLandingMntAbsentMarker
)

// verifyNixAbsentOutput asserts the nix absence probe: the target must show
// neither a nix binary nor a /nix tree, so the post-deploy proof can only come
// from the bootstrap install step. Detecting core `nix` needs no login shell
// (the product probes it bare, nixprobe.go); `su -l` here only matches the
// sibling probes.
func verifyNixAbsentOutput(output string) error {
	if strings.Contains(output, nixBinPresentMarker) || strings.Contains(output, nixDirPresentMarker) {
		return errors.Errorf("target must start without Nix for the bootstrap scenario, got: %q", output)
	}

	if !strings.Contains(output, nixBinAbsentMarker) || !strings.Contains(output, nixDirAbsentMarker) {
		return errors.Errorf("unexpected nix absence probe output (want %q and %q): %q",
			nixBinAbsentMarker, nixDirAbsentMarker, output)
	}

	return nil
}

// verifyNixAbsent pins the premise of the nix-install bootstrap scenario on
// the target itself.
func verifyNixAbsent(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	output, err := sshRun(port, keyPath, "su -l root -c '"+nixProbeBinSection+"'; "+nixProbeDirSection)
	if err != nil {
		return errors.Wrapf(err, "verify nix absence on %s", addr)
	}

	return errors.Wrapf(verifyNixAbsentOutput(output), "verify nix absence on %s", addr)
}

// verifyNixInstalledOutput asserts the nix-install bootstrap end state: the
// nix binary runs (command-exit marker) and the system-manager hello binary
// resolves into the /nix/store closure, i.e. the transferred closure is
// materialized in the store.
func verifyNixInstalledOutput(versionSection, helloSection string) error {
	if !strings.Contains(versionSection, nixBinOKMarker) {
		return errors.Errorf("nix --version failed after the nix-install bootstrap (want %q): %q",
			nixBinOKMarker, strings.TrimSpace(versionSection))
	}

	helloPath := strings.TrimSpace(helloSection)
	if !strings.HasPrefix(helloPath, nixStorePathPrefix) {
		return errors.Errorf("system-manager closure does not resolve under %s: %q", nixStorePathPrefix, helloPath)
	}

	return nil
}

// verifyNixInstalled asserts the nix-install bootstrap end state on a target
// that started without Nix.
func verifyNixInstalled(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// The product probes `nix --version` bare over plain SSH (nixprobe.go), so
	// no login shell is required for it. `su -l` here only matches
	// verifyPackage, which needs it to reach nix-profile binaries (not on the
	// plain SSH PATH). || true on the readlink keeps the probe running so both
	// sections are reported even when one fails.
	output, err := sshRun(port, keyPath,
		"su -l root -c '"+nixProbeVersionSection+"'; echo '---'; "+
			"readlink -f /run/system-manager/sw/bin/hello 2>/dev/null || true")
	if err != nil {
		return errors.Wrapf(err, "verify nix-install bootstrap on %s", addr)
	}

	parts := strings.SplitN(output, "---", splitParts)
	if len(parts) != splitParts {
		return errors.Errorf("unexpected nix-install bootstrap output on %s: %q", addr, output)
	}

	return errors.Wrapf(
		verifyNixInstalledOutput(parts[0], strings.TrimSpace(parts[1])),
		"verify nix-install bootstrap on %s", addr)
}

// verifySecretLandingOutput asserts the nix-install secret landing probe: the
// bootstrapping-root section must report the copy absent (no redirect for the
// nix-install bootstrap) and the plain section must carry the exact secret
// content on the live root. Sections are split like verifySecret: the raw cat
// bytes keep their trailing newline so the content check stays content-exact.
func verifySecretLandingOutput(output, wantContent string) error {
	parts := strings.SplitN(output, "---", secretVerifyParts)
	if len(parts) != secretVerifyParts {
		return errors.Errorf("unexpected secret landing probe output: %q", output)
	}

	mntSection := strings.TrimPrefix(parts[1], "\n")
	if strings.Contains(mntSection, secretLandingMntPresentMarker) {
		return errors.Errorf("secret must not land under the bootstrapping root %s (want %q): %q",
			bootstrappingRootPath, secretLandingMntAbsentMarker, strings.TrimSpace(mntSection))
	}

	if !strings.Contains(mntSection, secretLandingMntAbsentMarker) {
		return errors.Errorf("unexpected secret landing probe output (want %q): %q",
			secretLandingMntAbsentMarker, strings.TrimSpace(mntSection))
	}

	content := strings.TrimPrefix(parts[0], "\n")
	if strings.Contains(content, secretLandingPlainMissingMarker) {
		return errors.Errorf("secret missing at %s on the live root (want %q): %q",
			nixinstallSecretRemotePath, wantContent, strings.TrimSpace(content))
	}

	if content != wantContent {
		return errors.Errorf("secret %s content mismatch on the live root: got %q, want %q",
			nixinstallSecretRemotePath, content, wantContent)
	}

	return nil
}

// verifySecretLanding asserts the secret landing end state on the target: the
// OS secret sits on the live root at the plain remote path with the exact
// fixture content and no copy exists under the bootstrapping root.
func verifySecretLanding(port int, keyPath string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	wantContent, err := readSecretFixture(nixinstallSecretFixtureName)
	if err != nil {
		return err
	}

	output, err := sshRun(port, keyPath,
		secretLandingPlainProbeSection+"; echo '---'; "+secretLandingMntProbeSection)
	if err != nil {
		return errors.Wrapf(err, "verify secret landing on %s", addr)
	}

	return errors.Wrapf(verifySecretLandingOutput(output, wantContent),
		"verify secret landing on %s", addr)
}

// verifySystemManagerNixInstall asserts the nix-install bootstrap scenario
// end to end on the nix-less VM: the system-manager profile is active, the
// bootstrap left a working Nix with the closure in /nix/store, and the OS
// secret landed on the live root without a bootstrapping-root redirect.
func verifySystemManagerNixInstall(keyPath string) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		parGroup.Go("Verify system-manager on Debian nonix VM", func() error {
			return verifySystemManager(debianNonixVMPort, keyPath)
		})
		parGroup.Go("Verify Nix bootstrapped on Debian nonix VM", func() error {
			return verifyNixInstalled(debianNonixVMPort, keyPath)
		})
		parGroup.Go("Verify secret landing on Debian nonix VM", func() error {
			return verifySecretLanding(debianNonixVMPort, keyPath)
		})
	}

	return parGroup.Wait()
}

const (
	// secretsRemoteRoot is where panix.yml transfers every secret fixture.
	// The path is persistent across reboots: during install the bootstrapping
	// prefix writes it into the target root, so it reappears at the final path.
	//nolint:gosec // not a credential: the shared destination root for fixtures
	secretsRemoteRoot = "/var/lib/panix-e2e"

	secretVerifyParts = 2

	secretStatFieldCount = 3

	// tamperedSecretName is the command-sourced secret whose mode is broken
	// before the skip proof rerun: the probe must restore it without a write.
	tamperedSecretName = "command"
	tamperedSecretMode = "0644"
)

// e2eSecretExpectation is one transferred secret fixture; name maps to the
// remote path under secretsRemoteRoot.
type e2eSecretExpectation struct {
	name    string
	content string
	mode    string
}

// readSecretFixture reads a committed secret fixture from testflakes: the
// fixture file is the single source of truth for the transferred bytes, so
// panix.yml ships exactly these and the verifiers assert exactly these.
func readSecretFixture(name string) (string, error) {
	fixturePath := filepath.Join(findProjectRoot(), "tests", "e2e", "testflakes", name)

	fixture, err := os.ReadFile(fixturePath) //nolint:gosec // repo-local fixture
	if err != nil {
		return "", errors.Wrapf(err, "read secret fixture %s", fixturePath)
	}

	return string(fixture), nil
}

// e2eSecretExpectations returns every secret panix.yml transfers. The
// local_path fixture content is read from the committed fixture so config and
// verification share one source of truth.
func e2eSecretExpectations() ([]e2eSecretExpectation, error) {
	fixtureContent, err := readSecretFixture("secret-fixture.txt")
	if err != nil {
		return nil, err
	}

	return []e2eSecretExpectation{
		{name: "local-path", content: fixtureContent, mode: "640"},
		{name: "command", content: e2eCommandSecretContent, mode: "600"},
		{name: "command-local-path", content: fixtureContent, mode: "600"},
		{name: "age", content: e2eAgePlainContent, mode: "600"},
		{name: "sops", content: e2eSopsPlainContent, mode: "600"},
	}, nil
}

// commandSecretNames lists the command-sourced secrets: the sha256
// conditional-write logic applies to them (local_path-only sources go through
// rsync instead).
func commandSecretNames() []string {
	return []string{"command", "command-local-path", "age", "sops"}
}

// verifySecrets checks content and mode of every transferred secret fixture on
// the given port.
func verifySecrets(port int, keyPath string) error {
	expectations, err := e2eSecretExpectations()
	if err != nil {
		return err
	}

	for _, expected := range expectations {
		err = verifySecret(port, keyPath, expected)
		if err != nil {
			return err
		}
	}

	return nil
}

func verifySecret(port int, keyPath string, expected e2eSecretExpectation) error {
	path := filepath.Join(secretsRemoteRoot, expected.name)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	output, err := sshRun(port, keyPath, fmt.Sprintf("stat -c '%%a' -- %s; echo '---'; cat -- %s", path, path))
	if err != nil {
		return errors.Wrapf(err, "verify secret %s on %s", expected.name, addr)
	}

	parts := strings.SplitN(output, "---", secretVerifyParts)
	if len(parts) != secretVerifyParts {
		return errors.Errorf("unexpected secret verify output on %s for %s", addr, expected.name)
	}

	mode := strings.TrimSpace(parts[0])
	if mode != expected.mode {
		return errors.Errorf("secret %s on %s has mode %s, want %s", expected.name, addr, mode, expected.mode)
	}

	content := strings.TrimPrefix(parts[1], "\n")
	if content != expected.content {
		return errors.Errorf("secret %s on %s content mismatch: got %q, want %q", expected.name, addr, content, expected.content)
	}

	return nil
}

// secretStat is a secret's identity on the target: inode, mtime in epoch
// seconds, mode and content hash. A sha256 skip leaves all four untouched.
type secretStat struct {
	inode string
	mtime string
	mode  string
	hash  string
}

func readSecretStat(port int, keyPath, path string) (secretStat, error) {
	output, err := sshRun(port, keyPath, fmt.Sprintf("stat -c '%%i %%Y %%a' -- %s; sha256sum -- %s", path, path))
	if err != nil {
		return secretStat{}, errors.Wrapf(err, "read secret stat %s", path)
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != secretVerifyParts {
		return secretStat{}, errors.Errorf("unexpected stat output for %s: %q", path, output)
	}

	fields := strings.Fields(lines[0])
	if len(fields) != secretStatFieldCount {
		return secretStat{}, errors.Errorf("unexpected stat fields for %s: %q", path, lines[0])
	}

	return secretStat{inode: fields[0], mtime: fields[1], mode: fields[2], hash: strings.TrimSpace(lines[1])}, nil
}

// secretVerifyTarget is one VM receiving secrets for the active test scope.
type secretVerifyTarget struct {
	label string
	port  int
}

func secretVerifyTargets() []secretVerifyTarget {
	var targets []secretVerifyTarget

	if testScopeFlag.local() {
		targets = append(targets, secretVerifyTarget{label: "NixOS ISO VM", port: nixosISOPort})
	}

	if testScopeFlag.remote() {
		targets = append(targets, secretVerifyTarget{label: "remote ISO VM", port: remoteISOPort})
	}

	return targets
}

// verifySecretsConditionalSkip proves the sha256 conditional write end to end:
// a second Inspect+Secrets run with identical command output must skip every
// write, so inode, mtime and content hash stay identical, while the probe
// still enforces a tampered mode without rewriting the file.
func verifySecretsConditionalSkip(configPath string, res *testResources) error {
	printPhasef("Phase: Secrets conditional skip")

	targets := secretVerifyTargets()

	before := make(map[string]map[string]secretStat, len(targets))

	for _, target := range targets {
		stats, err := readCommandSecretStats(target.port, res.keyPath)
		if err != nil {
			return errors.Wrapf(err, "capture secret state on %s", target.label)
		}

		before[target.label] = stats

		tamperedPath := filepath.Join(secretsRemoteRoot, tamperedSecretName)

		_, err = sshRun(target.port, res.keyPath, "chmod "+tamperedSecretMode+" -- "+tamperedPath)
		if err != nil {
			return errors.Wrapf(err, "tamper with %s on %s", tamperedPath, target.label)
		}

		fmt.Printf("  %s: broke %s to mode %s before the rerun\n", target.label, tamperedPath, tamperedSecretMode)
	}

	err := runPanixSecretsStepWithArgs("Run panix secrets (skip proof)", configPath,
		[]string{"--tags", "test-vm,test-vm-kexec,test-vm-remote"},
		"PANIX_TEST_MODE=deploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
		"PANIX_NIX_INSTALLER_PATH="+res.nixInstallerPath,
	)
	if err != nil {
		return err
	}

	for _, target := range targets {
		err = verifyCommandSecretsUnchanged(target, res.keyPath, before[target.label])
		if err != nil {
			return err
		}

		fmt.Printf("  %s: all command-sourced secrets skipped (inode, mtime and content unchanged)\n", target.label)
	}

	return nil
}

func readCommandSecretStats(port int, keyPath string) (map[string]secretStat, error) {
	stats := make(map[string]secretStat)

	for _, name := range commandSecretNames() {
		path := filepath.Join(secretsRemoteRoot, name)

		stat, err := readSecretStat(port, keyPath, path)
		if err != nil {
			return nil, err
		}

		stats[name] = stat
	}

	return stats, nil
}

func verifyCommandSecretsUnchanged(target secretVerifyTarget, keyPath string, before map[string]secretStat) error {
	expectations, err := e2eSecretExpectations()
	if err != nil {
		return err
	}

	for _, expected := range expectations {
		previous, tracked := before[expected.name]
		if !tracked {
			continue
		}

		path := filepath.Join(secretsRemoteRoot, expected.name)

		after, statErr := readSecretStat(target.port, keyPath, path)
		if statErr != nil {
			return statErr
		}

		if after.inode != previous.inode {
			return errors.Errorf("secret %s on %s inode changed: %s -> %s", expected.name, target.label, previous.inode, after.inode)
		}

		if after.mtime != previous.mtime {
			return errors.Errorf("secret %s on %s mtime changed: %s -> %s (write was not skipped)",
				expected.name, target.label, previous.mtime, after.mtime)
		}

		if after.hash != previous.hash {
			return errors.Errorf("secret %s on %s content changed: %s -> %s", expected.name, target.label, previous.hash, after.hash)
		}

		if after.mode != expected.mode {
			return errors.Errorf("secret %s on %s has mode %s, want %s after probe enforcement",
				expected.name, target.label, after.mode, expected.mode)
		}
	}

	return nil
}
