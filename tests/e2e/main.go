package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

type testScope string

const (
	testScopeLocal  testScope = "local"
	testScopeRemote testScope = "remote"
	testScopeBoth   testScope = "both"
)

type deployPhase string

const (
	phaseAll       deployPhase = "all"
	phaseBootstrap deployPhase = "bootstrap"
	phaseRedeploy  deployPhase = "redeploy"
)

type redeployType string

const (
	redeployAll   redeployType = "all"
	redeployNixos redeployType = "nixos"
	redeployHome  redeployType = "home"
)

var errInvalidTestScope = errors.New("invalid test scope, must be local, remote, or both")
var errInvalidPhase = errors.New("invalid phase, must be: all, bootstrap, or redeploy")
var errInvalidRedeployType = errors.New("invalid redeploy-type, must be: all, nixos, or home")

var (
	testScopeFlag    testScope
	phaseFlag        deployPhase
	redeployTypeFlag redeployType
)

func parseFlags() {
	flag.Func("test", "test scope: local, remote, both (default: both)", func(val string) error {
		switch testScope(val) {
		case testScopeLocal, testScopeRemote, testScopeBoth:
			testScopeFlag = testScope(val)

			return nil
		default:
			return fmt.Errorf("%w: %q", errInvalidTestScope, val)
		}
	})

	flag.Func("phase", "deploy phase to run: all, bootstrap, redeploy (default: all)", func(val string) error {
		switch deployPhase(val) {
		case phaseAll, phaseBootstrap, phaseRedeploy:
			phaseFlag = deployPhase(val)

			return nil
		default:
			return fmt.Errorf("%w: %q", errInvalidPhase, val)
		}
	})

	flag.Func("redeploy-type", "which output types to redeploy: all, nixos, home (default: all)", func(val string) error {
		switch redeployType(val) {
		case redeployAll, redeployNixos, redeployHome:
			redeployTypeFlag = redeployType(val)

			return nil
		default:
			return fmt.Errorf("%w: %q", errInvalidRedeployType, val)
		}
	})

	flag.Parse()

	if testScopeFlag == "" {
		testScopeFlag = testScopeBoth
	}

	if phaseFlag == "" {
		phaseFlag = phaseAll
	}

	if redeployTypeFlag == "" {
		redeployTypeFlag = redeployAll
	}
}

func (s testScope) local() bool  { return s == testScopeBoth || s == testScopeLocal }
func (s testScope) remote() bool { return s == testScopeBoth || s == testScopeRemote }

const (
	blankDiskName       = "blank.qcow2"
	blankDiskSize       = "10G"
	blankDiskRemoteName = "blank-remote.qcow2"
	overlayName         = "debian-overlay.qcow2"
	overlayRemoteName   = "debian-overlay-remote.qcow2"
	overlayNixName      = "debian-overlay-nix.qcow2"
	nixosISOPort        = 10022
	kexecVMPort         = 10023
	remoteISOPort       = 10025
	remoteKexecPort     = 10026
	debianNixVMPort     = 10027
)

func main() {
	parseFlags()

	err := run()
	if err != nil {
		_ = writeBadgeSVG(false)

		failAndExit(err)
	}
}

func killStaleQEMU() {
	_ = exec.CommandContext(context.Background(), "pkill", "-f", "qemu-system-x86_64").Run()
}

func cleanupFifos() {
	matches, _ := filepath.Glob(filepath.Join(logDirPath, "*-serial.fifo"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

func run() error {
	testStart = time.Now()

	killStaleQEMU()

	cacheServer, err := startNixCacheServer()
	if err != nil {
		return err
	}

	defer func() {
		if cacheServer != nil && cacheServer.Process != nil {
			_ = cacheServer.Process.Kill()
			_ = cacheServer.Wait()
		}
	}()

	err = runChecks()
	if err != nil {
		return err
	}

	projectRoot := findProjectRoot()
	configPath := projectRoot + "/tests/e2e/panix.yml"

	res, err := phase0Setup()
	if err != nil {
		return err
	}

	vms, err := startTestVMs(res)
	if err != nil {
		return err
	}

	defer vms.kill()
	defer cleanupFifos()

	err = runDeployPhases(configPath, res)
	if err != nil {
		return err
	}

	printFinalf("All tests passed!")

	_ = writeBadgeSVG(true)

	return nil
}

func runDeployPhases(configPath string, res *testResources) error {
	if phaseFlag == phaseBootstrap || phaseFlag == phaseAll {
		err := runBootstrapPhase(configPath, res)
		if err != nil {
			return err
		}
	}

	if phaseFlag == phaseRedeploy || phaseFlag == phaseAll {
		err := runRedeployPhase(configPath, res)
		if err != nil {
			return err
		}
	}

	return nil
}

func runBootstrapPhase(configPath string, res *testResources) error {
	printPhasef("Phase: Bootstrap deploy")

	err := runPanixDeployStep("Run panix deploy", configPath,
		"PANIX_TEST_MODE=bootstrap",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	if err != nil {
		return err
	}

	return verifyAll(res.keyPath)
}

func runRedeployPhase(configPath string, res *testResources) error {
	if redeployTypeFlag == redeployAll || redeployTypeFlag == redeployNixos {
		err := runRedeployNixOS(configPath, res)
		if err != nil {
			return err
		}
	}

	if redeployTypeFlag == redeployAll || redeployTypeFlag == redeployHome {
		err := runRedeployHome(configPath, res)
		if err != nil {
			return err
		}
	}

	return nil
}

func runRedeployNixOS(configPath string, res *testResources) error {
	printPhasef("Phase: Redeploy NixOS")

	err := runPanixDeployStepWithArgs("Run panix deploy (nixos)", configPath,
		[]string{"--tags", "nixosConfigurations"},
		"PANIX_TEST_MODE=redeploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	if err != nil {
		return err
	}

	return verifyAll(res.keyPath)
}

func runRedeployHome(configPath string, res *testResources) error {
	printPhasef("Phase: Redeploy home-manager")

	err := runPanixDeployStepWithArgs("Run panix deploy (home-manager)", configPath,
		[]string{"--tags", "homeConfigurations"},
		"PANIX_TEST_MODE=redeploy",
		"PANIX_TEST_SCOPE="+string(testScopeFlag),
		"PANIX_KEXEC_PATH="+res.kexecInstallerPath,
	)
	if err != nil {
		return err
	}

	if testScopeFlag.local() {
		return verifyHomeManager(res.keyPath)
	}

	return nil
}

func runChecks() error {
	err := checkKVM()
	if err != nil {
		return err
	}

	err = checkDeps()
	if err != nil {
		return err
	}

	return initDirs()
}

type testResources struct {
	keyPath             string
	installerISOPath    string
	kexecInstallerPath  string
	cloudInitSeed       string
	cloudInitSeedRemote string
	cloudInitSeedNix    string
	debianImagePath     string
	debianNixImagePath  string
	debianOverlay       string
	debianOverlayRemote string
	debianOverlayNix    string
	blankDisk           string
	blankDiskRemote     string
}

func phase0Setup() (*testResources, error) {
	printPhasef("Phase 0: Setup - prepare artifacts")

	res := &testResources{}

	parGroup := newParallelGroup()
	parGroup.Go("Resolve SSH keys", func() error {
		var keyErr error

		res.keyPath, keyErr = ensureSSHKeys()

		return keyErr
	})

	err := parGroup.Wait()
	if err != nil {
		return nil, err
	}

	res.debianImagePath, err = bakeStep("Bake Debian image (rsync pre-install)")
	if err != nil {
		return nil, err
	}

	if testScopeFlag.local() {
		err = simpleStep("Bake Debian image (nix pre-install)", func() error {
			var bakeErr error

			res.debianNixImagePath, bakeErr = bakeDebianNixImage()

			return bakeErr
		})
		if err != nil {
			return nil, err
		}
	}

	err = buildNixArtifacts(res)
	if err != nil {
		return nil, err
	}

	err = createDisks(res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func buildNixArtifacts(res *testResources) error {
	parGroup := newParallelGroup()
	parGroup.Go("Build kexec installer", func() error {
		var buildErr error

		res.kexecInstallerPath, buildErr = buildKexecInstaller()

		return buildErr
	})
	parGroup.Go("Build NixOS installer ISO", func() error {
		var buildErr error

		res.installerISOPath, buildErr = buildInstallerISO()

		return buildErr
	})
	parGroup.Go("Pre-build test-vm closure", func() error {
		return preBuildClosure("test-vm", "nixosConfigurations.test-vm.config.system.build.toplevel")
	})

	if testScopeFlag.local() {
		parGroup.Go("Pre-build home-manager closure", func() error {
			return preBuildClosure("home-manager", "homeConfigurations.test-home.activationPackage")
		})
		parGroup.Go("Pre-build home-manager (alice) closure", func() error {
			return preBuildClosure("home-manager-alice", "homeConfigurations.test-home-alice.activationPackage")
		})
	}

	if testScopeFlag.remote() {
		parGroup.Go("Pre-build test-vm-remote closure", func() error {
			return preBuildClosure("test-vm-remote", "nixosConfigurations.test-vm-remote.config.system.build.toplevel")
		})
		parGroup.Go("Pre-build test-vm-remote disko script", func() error {
			return preBuildClosure("test-vm-remote disko", "nixosConfigurations.test-vm-remote.config.system.build.diskoScript")
		})
	}

	return parGroup.Wait()
}

func createDisks(res *testResources) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		createLocalDisks(parGroup, res)
	}

	if testScopeFlag.remote() {
		createRemoteDisks(parGroup, res)
	}

	return parGroup.Wait()
}

func createLocalDisks(parGroup *parallelGroup, res *testResources) {
	parGroup.Go("Create cloud-init seed", func() error {
		var seedErr error

		res.cloudInitSeed, seedErr = nixBuild("seed-iso")

		return seedErr
	})
	parGroup.Go("Create Debian overlay disk", func() error {
		var diskErr error

		res.debianOverlay, diskErr = createDisk(overlayName, "-b", res.debianImagePath, "-F", "qcow2")

		return diskErr
	})
	parGroup.Go("Create blank disk", func() error {
		var diskErr error

		res.blankDisk, diskErr = createDisk(blankDiskName, blankDiskSize)

		return diskErr
	})
	parGroup.Go("Create Debian-nix cloud-init seed", func() error {
		var seedErr error

		// Simple SSH-only seed — nix is already baked into the image.
		res.cloudInitSeedNix, seedErr = nixBuild("seed-nix-iso")

		return seedErr
	})
	parGroup.Go("Create Debian-nix overlay disk", func() error {
		var diskErr error

		res.debianOverlayNix, diskErr = createDisk(overlayNixName, "-b", res.debianNixImagePath, "-F", "qcow2")

		return diskErr
	})
}

func createRemoteDisks(parGroup *parallelGroup, res *testResources) {
	parGroup.Go("Create cloud-init seed (remote)", func() error {
		var seedErr error

		res.cloudInitSeedRemote, seedErr = nixBuild("seed-remote-iso")

		return seedErr
	})
	parGroup.Go("Create Debian overlay disk (remote)", func() error {
		var diskErr error

		res.debianOverlayRemote, diskErr = createDisk(overlayRemoteName, "-b", res.debianImagePath, "-F", "qcow2")

		return diskErr
	})
	parGroup.Go("Create blank disk (remote)", func() error {
		var diskErr error

		res.blankDiskRemote, diskErr = createDisk(blankDiskRemoteName, blankDiskSize)

		return diskErr
	})
}

type testVMs struct {
	isoVM         *qemuVM
	kexecVM       *qemuVM
	remoteISOVM   *qemuVM
	remoteKexecVM *qemuVM
	debianNixVM   *qemuVM
}

func startTestVMs(res *testResources) (*testVMs, error) {
	vms := &testVMs{}

	var err error

	if testScopeFlag.local() {
		err = startLocalVMs(vms, res)
		if err != nil {
			return nil, err
		}
	}

	if testScopeFlag.remote() {
		err = startRemoteVMs(vms, res)
		if err != nil {
			return nil, err
		}
	}

	err = waitForAllSSH(res.keyPath)
	if err != nil {
		return nil, err
	}

	return vms, nil
}

func startLocalVMs(vms *testVMs, res *testResources) error {
	var err error

	vms.isoVM, err = startVMStep("Start NixOS ISO VM (port %d)", nixosISOPort, "iso-vm",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,cache=unsafe", res.blankDisk),
		"-cdrom", res.installerISOPath,
	)
	if err != nil {
		return err
	}

	vms.kexecVM, err = startVMStep("Start kexec VM (port %d)", kexecVMPort, "kexec-vm",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,cache=unsafe", res.debianOverlay),
		"-cdrom", res.cloudInitSeed,
	)
	if err != nil {
		vms.kill()

		return err
	}

	vms.debianNixVM, err = startVMStep("Start Debian-nix VM (port %d)", debianNixVMPort, "debian-nix-vm",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,cache=unsafe", res.debianOverlayNix),
		"-cdrom", res.cloudInitSeedNix,
	)
	if err != nil {
		vms.kill()

		return err
	}

	return nil
}

func startRemoteVMs(vms *testVMs, res *testResources) error {
	var err error

	vms.remoteISOVM, err = startVMStep("Start remote NixOS ISO VM (port %d)", remoteISOPort, "iso-vm-remote",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,cache=unsafe", res.blankDiskRemote),
		"-cdrom", res.installerISOPath,
	)
	if err != nil {
		vms.kill()

		return err
	}

	vms.remoteKexecVM, err = startVMStep("Start remote kexec VM (port %d)", remoteKexecPort, "kexec-vm-remote",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,cache=unsafe", res.debianOverlayRemote),
		"-cdrom", res.cloudInitSeedRemote,
	)
	if err != nil {
		vms.kill()

		return err
	}

	return nil
}

func (vms *testVMs) kill() {
	if vms.isoVM != nil {
		vms.isoVM.kill()
	}

	if vms.kexecVM != nil {
		vms.kexecVM.kill()
	}

	if vms.remoteISOVM != nil {
		vms.remoteISOVM.kill()
	}

	if vms.remoteKexecVM != nil {
		vms.remoteKexecVM.kill()
	}

	if vms.debianNixVM != nil {
		vms.debianNixVM.kill()
	}
}

func waitForAllSSH(keyPath string) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		parGroup.Go("Wait for SSH on NixOS ISO VM", func() error {
			return waitForSSH(nixosISOPort, keyPath)
		})
		parGroup.Go("Wait for SSH on kexec VM", func() error {
			return waitForSSH(kexecVMPort, keyPath)
		})
		parGroup.Go("Wait for SSH on Debian-nix VM", func() error {
			return waitForSSH(debianNixVMPort, keyPath)
		})
	}

	if testScopeFlag.remote() {
		parGroup.Go("Wait for SSH on remote NixOS ISO VM", func() error {
			return waitForSSH(remoteISOPort, keyPath)
		})
		parGroup.Go("Wait for SSH on remote kexec VM", func() error {
			return waitForSSH(remoteKexecPort, keyPath)
		})
	}

	return parGroup.Wait()
}

func verifyAll(keyPath string) error {
	parGroup := newParallelGroup()

	if testScopeFlag.local() {
		parGroup.Go("Verify NixOS on ISO VM", func() error {
			return verifyNixOSInstallation(nixosISOPort, keyPath)
		})
		parGroup.Go("Verify NixOS on kexec VM", func() error {
			return verifyNixOSInstallation(kexecVMPort, keyPath)
		})
	}

	if testScopeFlag.remote() {
		parGroup.Go("Verify NixOS on remote ISO VM", func() error {
			return verifyNixOSInstallation(remoteISOPort, keyPath)
		})
		parGroup.Go("Verify NixOS on remote kexec VM", func() error {
			return verifyNixOSInstallation(remoteKexecPort, keyPath)
		})
	}

	return parGroup.Wait()
}

func startVMStep(format string, port int, logName string, extraArgs ...string) (*qemuVM, error) {
	step := startStep(format, port)

	guest, err := startQEMU(logName, port, extraArgs...)
	if err != nil {
		step.Fail(err)

		return nil, err
	}

	step.Done()

	return guest, nil
}

func bakeStep(name string) (string, error) {
	step := startStep("%s", name)

	result, err := bakeDebianImage()
	if err != nil {
		step.Fail(err)

		return "", err
	}

	step.Done()

	return result, nil
}

func simpleStep(name string, fn func() error) error {
	step := startStep("%s", name)

	err := fn()
	if err != nil {
		step.Fail(err)

		return err
	}

	step.Done()

	return nil
}

func runPanixDeployStep(name, configPath string, envVars ...string) error {
	return runPanixDeployStepWithArgs(name, configPath, nil, envVars...)
}

func runPanixDeployStepWithArgs(name, configPath string, extraArgs []string, envVars ...string) error {
	start := time.Now()

	fmt.Printf("  → %s\n", name)

	err := runPanixDeployWithArgs(configPath, extraArgs, envVars...)

	elapsed := formatElapsed(time.Since(start))

	if err != nil {
		mode := envValue(envVars, "PANIX_TEST_MODE")
		if mode == "" {
			mode = "default"
		}

		fmt.Printf("  ✗ %s  FAILED (%s): %s\n", name, elapsed, err)
		fmt.Printf("    see logs in: %s/panix-%s.*.log\n", logDirPath, mode)

		return err
	}

	fmt.Printf("  ✓ %s  %s\n", name, elapsed)

	return nil
}
