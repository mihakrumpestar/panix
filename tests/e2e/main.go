package main

import (
	"fmt"
	"time"
)

const (
	kexecFileName = "kexec-installer-x86_64-linux.tar.gz"
	kexecURL      = "https://github.com/nix-community/nixos-images/releases/latest/download/nixos-kexec-installer-noninteractive-x86_64-linux.tar.gz"
	debianURL     = "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2"
	blankDiskName = "blank.qcow2"
	blankDiskSize = "10G"
	seedName      = "seed.iso"
	overlayName   = "debian-overlay.qcow2"
	nixosISOPort  = 10022
	kexecVMPort   = 10023
)

func main() {
	err := run()
	if err != nil {
		_ = writeBadgeSVG(false)

		failAndExit(err)
	}
}

func run() error {
	testStart = time.Now()

	err := runChecks()
	if err != nil {
		return err
	}

	projectRoot := findProjectRoot()
	configPath := projectRoot + "/tests/e2e/panix.yml"

	res, err := phase0Setup()
	if err != nil {
		return err
	}

	isoVM, kexecVM, err := startTestVMs(res)
	if err != nil {
		return err
	}

	defer isoVM.kill()
	defer kexecVM.kill()

	err = runPhase(configPath, "Bootstrap", "PANIX_TEST_MODE=bootstrap", res.keyPath)
	if err != nil {
		return err
	}

	err = runPhase(configPath, "Redeploy", "PANIX_TEST_MODE=redeploy", res.keyPath)
	if err != nil {
		return err
	}

	printFinalf("All tests passed!")

	_ = writeBadgeSVG(true)

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
	keyPath          string
	installerISOPath string
	cloudInitSeed    string
	debianImagePath  string
	debianOverlay    string
	blankDisk        string
}

func phase0Setup() (*testResources, error) {
	printPhasef("Phase 0: Setup - prepare artifacts")

	res := &testResources{}

	parGroup := newParallelGroup()
	parGroup.Go("Generate SSH keys", func() error {
		var keyErr error

		res.keyPath, keyErr = ensureSSHKeys()

		return keyErr
	})
	parGroup.Go("Download kexec image", func() error {
		return ensureCached(kexecFileName, kexecURL)
	})
	parGroup.Go("Download Debian image", func() error {
		return ensureCached(debianFileName, debianURL)
	})

	err := parGroup.Wait()
	if err != nil {
		return nil, err
	}

	res.debianImagePath, err = bakeStep("Bake Debian image (rsync pre-install)", res.keyPath)
	if err != nil {
		return nil, err
	}

	err = simpleStep("Stage SSH public key", func() error { return stageSSHPubKey(res.keyPath) })
	if err != nil {
		return nil, err
	}

	res.installerISOPath, err = buildStep("Build NixOS installer ISO", buildInstallerISO)
	if err != nil {
		return nil, err
	}

	err = createDisks(res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func createDisks(res *testResources) error {
	parGroup := newParallelGroup()
	parGroup.Go("Create cloud-init seed", func() error {
		var seedErr error

		res.cloudInitSeed, seedErr = createCloudInitISO(seedName, "panix-kexec-test", "kexec-vm", res.keyPath+".pub", false)

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

	return parGroup.Wait()
}

func startTestVMs(res *testResources) (*qemuVM, *qemuVM, error) {
	isoVM, err := startVMStep("Start NixOS ISO VM (port %d)", nixosISOPort, "iso-vm",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,cache=unsafe", res.blankDisk),
		"-cdrom", res.installerISOPath,
	)
	if err != nil {
		return nil, nil, err
	}

	kexecVM, err := startVMStep("Start kexec VM (port %d)", kexecVMPort, "kexec-vm",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio,cache=unsafe", res.debianOverlay),
		"-cdrom", res.cloudInitSeed,
	)
	if err != nil {
		isoVM.kill()

		return nil, nil, err
	}

	parGroup := newParallelGroup()
	parGroup.Go("Wait for SSH on NixOS ISO VM", func() error {
		return waitForSSH(nixosISOPort, res.keyPath)
	})
	parGroup.Go("Wait for SSH on kexec VM", func() error {
		return waitForSSH(kexecVMPort, res.keyPath)
	})

	err = parGroup.Wait()
	if err != nil {
		return nil, nil, err
	}

	return isoVM, kexecVM, nil
}

func runPhase(configPath, phaseName, modeEnv, keyPath string) error {
	printPhasef("Phase: %s deploy", phaseName)

	step := startStep("%s", "Run panix "+phaseName+" deploy")

	err := runPanixDeploy(configPath, modeEnv)
	if err != nil {
		step.Fail(err)

		return err
	}

	step.Done()

	return verifyBoth(keyPath)
}

func verifyBoth(keyPath string) error {
	parGroup := newParallelGroup()
	parGroup.Go("Verify NixOS on ISO VM", func() error {
		return verifyNixOSInstallation(nixosISOPort, keyPath)
	})
	parGroup.Go("Verify NixOS on kexec VM", func() error {
		return verifyNixOSInstallation(kexecVMPort, keyPath)
	})

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

func bakeStep(name, keyPath string) (string, error) {
	step := startStep("%s", name)

	result, err := bakeDebianImage(keyPath)
	if err != nil {
		step.Fail(err)

		return "", err
	}

	step.Done()

	return result, nil
}

func buildStep(name string, fn func() (string, error)) (string, error) {
	step := startStep("%s", name)

	result, err := fn()
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
