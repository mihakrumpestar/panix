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

var errBakeTimeout = errors.New("bake VM timed out")

type qemuVM struct {
	cmd        *exec.Cmd
	fifoPath   string
	cancelRead context.CancelFunc
}

func startQEMU(logName string, hostFwdPort int, extraArgs ...string) (*qemuVM, error) {
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

	args := buildQEMUArgs(hostFwdPort, fifoPath, extraArgs...)
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

func buildQEMUArgs(hostFwdPort int, fifoPath string, extraArgs ...string) []string {
	args := []string{
		"-enable-kvm", "-cpu", "host",
		"-smp", vmCPUs, "-m", vmMemory,
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", hostFwdPort),
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

func bakeDebianImage() (string, error) {
	bakedPath := filepath.Join(cacheDirPath, "debian-baked.qcow2")
	markerPath := bakedPath + ".rsync-ok"

	_, markerErr := os.Stat(markerPath)
	if markerErr == nil {
		return bakedPath, nil
	}

	basePath, err := nixBuild("debian-cloud-image")
	if err != nil {
		return "", err
	}

	return bakeDebianWithSeed(bakedPath, basePath, "bake-seed-iso", "bake-vm", markerPath)
}

func bakeDebianNixImage() (string, error) {
	bakedPath := filepath.Join(cacheDirPath, "debian-baked-nix.qcow2")
	markerPath := bakedPath + ".nix-ok"

	_, markerErr := os.Stat(markerPath)
	if markerErr == nil {
		return bakedPath, nil
	}

	basePath, err := nixBuild("debian-cloud-image")
	if err != nil {
		return "", err
	}

	return bakeDebianWithSeed(bakedPath, basePath, "bake-seed-nix-iso", "bake-vm-nix", markerPath)
}

func bakeDebianWithSeed(bakedPath, basePath, seedAttr, hostname, markerPath string) (string, error) {
	_ = os.Remove(bakedPath)

	cpArgs := []string{"--reflink=auto", "--no-preserve=mode", basePath, bakedPath}

	out, err := exec.CommandContext(context.Background(), "cp", cpArgs...).CombinedOutput() //nolint:gosec
	if err != nil {
		return "", errors.Wrapf(err, "cp base image: %s", string(out))
	}

	seedPath, err := nixBuild(seedAttr)
	if err != nil {
		return "", err
	}

	consolePath := filepath.Join(logDirPath, hostname+"-console.log")
	cmd := exec.CommandContext(context.Background(), "qemu-system-x86_64", //nolint:gosec
		"-enable-kvm", "-cpu", "host", "-m", bakeVMMemory,
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", bakeVMPort),
		"-device", "virtio-net-pci,netdev=net0",
		"-serial", "file:"+consolePath,
		"-display", "none", "-nographic",
		"-drive", fmt.Sprintf("file=%s,format=qcow2,if=virtio", bakedPath),
		"-cdrom", seedPath,
	)

	err = cmd.Start()
	if err != nil {
		return "", errors.Wrap(err, "start bake VM")
	}

	done := make(chan error, 1)

	go func() { done <- cmd.Wait() }()

	select {
	case waitErr := <-done:
		if waitErr != nil {
			return "", errors.Wrap(waitErr, "bake VM exited with error")
		}

		writeErr := os.WriteFile(markerPath, []byte("ok"), pubFilePerm)
		if writeErr != nil {
			return "", errors.Wrap(writeErr, "write bake marker")
		}

		return bakedPath, nil
	case <-time.After(bakeTimeout):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()

		return "", errBakeTimeout
	}
}
