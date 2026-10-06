package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/errors"
)

const (
	vmMemory     = "4G"
	vmCPUs       = "4"
	bakeVMMemory = "2G"
	bakeVMPort   = 10024
	bakeTimeout  = 10 * time.Minute

	consoleLogName = "console.log"
)

// netRestrict marks whether QEMU user-net blocks guest outbound traffic
// (slirp restrict=on). Verified empirically: with restrict=on the guest still
// gets its DHCP lease and the SSH banner travels through the hostfwd port,
// while every other guest connection (internet, host services) is dropped.
type netRestrict bool

const (
	netOpen      netRestrict = false
	netAirGapped netRestrict = true
)

var errBakeTimeout = errors.New("bake VM timed out")

type qemuVM struct {
	cmd        *exec.Cmd
	fifoPath   string
	cancelRead context.CancelFunc
}

// userNetdev builds the single source of truth for the user-mode netdev:
// SSH hostfwd plus optional total guest isolation (restrict=on). The
// nix-less VM runs netAirGapped so its scenario is enforced at the QEMU
// level; VMs that bake images or substitute closures need netOpen.
func userNetdev(hostFwdPort int, restrictNet netRestrict) string {
	netdev := fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", hostFwdPort)

	if restrictNet == netAirGapped {
		netdev += ",restrict=on"
	}

	return netdev
}

func startQEMU(logName string, hostFwdPort int, restrictNet netRestrict, extraArgs ...string) (*qemuVM, error) {
	consolePath := filepath.Join(logDirPath, logName+"-"+consoleLogName)
	fifoPath := filepath.Join(logDirPath, logName+"-serial.fifo")

	_ = os.Remove(fifoPath)

	err := syscall.Mkfifo(fifoPath, filePerm)
	if err != nil {
		return nil, errors.Wrap(err, "create FIFO")
	}

	_, cancel, consoleFile, err := startConsoleLogger(fifoPath, consolePath)
	if err != nil {
		return nil, err
	}

	args := buildQEMUArgs(hostFwdPort, restrictNet, fifoPath, extraArgs...)
	cmd := exec.CommandContext(context.Background(), "qemu-system-x86_64", args...) //nolint:gosec

	err = cmd.Start()
	if err != nil {
		cancel()
		closeWithoutErrCheck(consoleFile)

		return nil, errors.Wrapf(err, "start QEMU %s", logName)
	}

	return &qemuVM{cmd: cmd, fifoPath: fifoPath, cancelRead: cancel}, nil
}

func startConsoleLogger(fifoPath, consolePath string) (context.Context, context.CancelFunc, *os.File, error) {
	consoleFile, err := os.Create(consolePath) //nolint:gosec
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "create console log")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer closeWithoutErrCheck(consoleFile)

		fifo, openErr := os.Open(fifoPath) //nolint:gosec
		if openErr != nil {
			return
		}

		defer closeWithoutErrCheck(fifo)

		scanner := bufio.NewScanner(fifo)

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ts := time.Now().Format("2006-01-02T15:04:05.000 ")

			_, writeErr := consoleFile.WriteString(ts + scanner.Text() + "\n")
			if writeErr != nil {
				return
			}
		}
	}()

	return ctx, cancel, consoleFile, nil
}

func buildQEMUArgs(hostFwdPort int, restrictNet netRestrict, fifoPath string, extraArgs ...string) []string {
	args := []string{
		"-enable-kvm", "-cpu", "host",
		"-smp", vmCPUs, "-m", vmMemory,
		"-netdev", userNetdev(hostFwdPort, restrictNet),
		"-device", "virtio-net-pci,netdev=net0",
		"-serial", "file:" + fifoPath,
		"-nographic", "-display", "none",
	}

	return append(args, extraArgs...)
}

func (vm *qemuVM) kill() {
	if vm == nil || vm.cmd == nil || vm.cmd.Process == nil {
		return
	}

	_ = vm.cmd.Process.Kill()
	_ = vm.cmd.Wait()

	if vm.cancelRead != nil {
		vm.cancelRead()
	}

	_ = os.Remove(vm.fifoPath)
}

func createDisk(name string, args ...string) (string, error) {
	path := filepath.Join(cacheDirPath, name)
	_ = os.Remove(path)

	allArgs := []string{"create", "-f", "qcow2", path}
	allArgs = append(allArgs, args...)

	out, err := exec.CommandContext(context.Background(), "qemu-img", allArgs...).CombinedOutput() //nolint:gosec
	if err != nil {
		return "", errors.Wrapf(err, "create %s: %s", name, string(out))
	}

	return path, nil
}

// bakeDebianImage bakes the plain Debian image (rsync and curl pre-installed,
// no Nix). It backs the kexec VMs and the nix-less Debian VM.
func bakeDebianImage() (string, error) {
	return bakeDebianWithSeed(filepath.Join(cacheDirPath, "debian-baked.qcow2"), "bake-seed-iso", "bake-vm")
}

// bakeDebianNixImage bakes the Debian image with Nix pre-installed (used by
// the Debian-nix VM).
func bakeDebianNixImage() (string, error) {
	return bakeDebianWithSeed(filepath.Join(cacheDirPath, "debian-baked-nix.qcow2"), "bake-seed-nix-iso", "bake-vm-nix")
}

// bakeDebianWithSeed bakes a Debian qcow2 through a cloud-init seed. The
// completion marker is keyed on the content-addressed base image and seed
// store paths, so changing either (e.g. the baked package list) forces a
// rebake instead of silently reusing a stale image.
func bakeDebianWithSeed(bakedPath, seedAttr, hostname string) (string, error) {
	basePath, err := nixBuild("debian-cloud-image")
	if err != nil {
		return "", err
	}

	seedPath, err := nixBuild(seedAttr)
	if err != nil {
		return "", err
	}

	markerPath := fmt.Sprintf("%s.%s.%s.ok", bakedPath, filepath.Base(basePath), filepath.Base(seedPath))

	_, markerErr := os.Stat(markerPath)
	if markerErr == nil {
		return bakedPath, nil
	}

	_ = os.Remove(bakedPath)

	cpArgs := []string{"--reflink=auto", "--no-preserve=mode", basePath, bakedPath}

	out, err := exec.CommandContext(context.Background(), "cp", cpArgs...).CombinedOutput() //nolint:gosec
	if err != nil {
		return "", errors.Wrapf(err, "cp base image: %s", string(out))
	}

	err = runBakeVM(bakedPath, seedPath, hostname)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(markerPath, []byte("ok"), pubFilePerm)
	if err != nil {
		return "", errors.Wrap(err, "write bake marker")
	}

	return bakedPath, nil
}

// runBakeVM boots the cloud-init bake seed against bakedPath and waits for the
// guest to shut itself down after the bake, failing on error or timeout. The
// bake VM needs outbound access (cloud-init installs packages, the withNix
// bake downloads the installer), so it runs netOpen.
func runBakeVM(bakedPath, seedPath, hostname string) error {
	consolePath := filepath.Join(logDirPath, hostname+"-console.log")
	cmd := exec.CommandContext(context.Background(), "qemu-system-x86_64", //nolint:gosec
		"-enable-kvm", "-cpu", "host", "-m", bakeVMMemory,
		"-netdev", userNetdev(bakeVMPort, netOpen),
		"-device", "virtio-net-pci,netdev=net0",
		"-serial", "file:"+consolePath,
		"-display", "none", "-nographic",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio", bakedPath),
		"-cdrom", seedPath,
	)

	err := cmd.Start()
	if err != nil {
		return errors.Wrap(err, "start bake VM")
	}

	done := make(chan error, 1)

	go func() { done <- cmd.Wait() }()

	select {
	case waitErr := <-done:
		return errors.Wrap(waitErr, "bake VM exited with error")
	case <-time.After(bakeTimeout):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()

		return errBakeTimeout
	}
}
