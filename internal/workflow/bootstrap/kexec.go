package bootstrap

import (
	"path"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/runtimevars"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/osrelease"
	"github.com/pkg/errors"
)

var ErrKexecBootFailed = errors.New("kexec did not boot into NixOS installer")

const (
	// kexecStagingPrefix names the per-run private staging directory.
	kexecStagingPrefix = "panix-kexec"
	// kexecTarballName is the downloaded tarball inside that directory.
	kexecTarballName = "kexec.tar"
)

func executeKexec(exc *executioner.Executioner, machineI *machine.Machine) error {
	kexecURL, err := resolveKexecURL(machineI)
	if err != nil {
		return err
	}

	stagingDir, err := createKexecDirectory(exc, machineI)
	if err != nil {
		return err
	}

	// Clean up only before kexec takes over: after the reboot the old
	// staging filesystem is gone and cleanup is neither needed nor practical.
	cleanup := func() { _ = removeStagingDir(exc, machineI, stagingDir) }

	err = downloadOrTransferKexec(exc, machineI, kexecURL, stagingDir)
	if err != nil {
		cleanup()

		return err
	}

	err = extractKexecTarball(exc, kexecURL, stagingDir)
	if err != nil {
		cleanup()

		return err
	}

	err = runKexecCommand(exc, machineI, stagingDir)
	if err != nil {
		cleanup()

		return err
	}

	err = waitForKexecReboot(exc, machineI)
	if err != nil {
		return err
	}

	// The active connection switched to the kexec installer: re-probe
	// rootness so later elevation matches the installer's user. A stale
	// non-root IsRoot would fail with "sudo: not found" (no sudo there).
	err = phaseops.RefreshSuperuser(exc, machineI)
	if err != nil {
		return err //nolint:wrapcheck // error is pre-annotated with its own context
	}

	machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
		mi.RequiresKexec = false
	})

	return nil
}

// resolveKexecURL expands PANIX_* runtime variables in the configured kexec
// image. The dry-run pseudo-architecture maps to x86_64 so dry-run plans stay
// explicit.
func resolveKexecURL(machineI *machine.Machine) (string, error) {
	arch := machineI.MetaInspect.Load().Architecture.String()

	if arch == "DRY_RUN" {
		arch = "x86_64"
	}

	image := machineI.Bootstrap.Kexec.Image.String()

	kexecURL, err := runtimevars.Vars{runtimevars.Arch: arch}.Expand(image)
	if err != nil {
		return "", errors.Wrap(err, "invalid kexec image")
	}

	return kexecURL, nil
}

// createKexecDirectory creates a unique per-run staging dir owned by the
// ACTIVE connection's user (bootstrap during kexec staging), not the regular
// SSH user, which may differ.
func createKexecDirectory(exc *executioner.Executioner, machineI *machine.Machine) (string, error) {
	stagingDir, err := newStagingDirPath(kexecStagingPrefix)
	if err != nil {
		return "", err
	}

	err = createPrivateStagingDir(exc, machineI, stagingDir)
	if err != nil {
		return "", err
	}

	return stagingDir, nil
}

func downloadOrTransferKexec(exc *executioner.Executioner, machineI *machine.Machine, kexecURL, stagingDir string) error {
	tarballPath := path.Join(stagingDir, kexecTarballName)

	return downloadOrTransfer(exc, machineI, kexecURL, tarballPath, "kexec tarball", machineI.Bootstrap.Kexec.GetCurlDefaultFlags())
}

// extractKexecTarball extracts the kexec tarball into the per-run staging dir.
func extractKexecTarball(exc *executioner.Executioner, kexecURL, stagingDir string) error {
	tarArgs := getTarArgs(kexecURL, path.Join(stagingDir, kexecTarballName))
	tarArgs = append(tarArgs, "-C", stagingDir)

	err := exc.Exec(
		"extract kexec tarball",
		"extracting kexec tarball",
		"failed to extract kexec tarball",
		append([]string{"tar"}, tarArgs...),
	)
	if err != nil {
		return errors.Wrap(err, "failed to extract kexec tarball")
	}

	return nil
}

// getTarArgs returns the appropriate tar extraction arguments based on file extension.
func getTarArgs(kexecURL, tarballPath string) []string {
	switch {
	case strings.HasSuffix(kexecURL, ".tar.gz") || strings.HasSuffix(kexecURL, ".tgz"):
		return []string{"-xzf", tarballPath}
	case strings.HasSuffix(kexecURL, ".tar.xz"):
		return []string{"-xJf", tarballPath}
	case strings.HasSuffix(kexecURL, ".tar.zst"):
		return []string{"--use-compress-program=zstd", "-xf", tarballPath}
	default:
		return []string{"-xf", tarballPath}
	}
}

// runKexecCommand executes the kexec script to boot into the NixOS installer.
func runKexecCommand(exc *executioner.Executioner, machineI *machine.Machine, stagingDir string) error {
	kexecCmd := append(machineI.MaybeSudo(), path.Join(stagingDir, "kexec", "run"))

	if len(machineI.Bootstrap.Kexec.ExtraFlags) != 0 {
		extraFlags := append([]string{"--kexec-extra-flags"}, machineI.Bootstrap.Kexec.ExtraFlags...)

		kexecCmd = append(kexecCmd, extraFlags...)
	}

	err := exc.Exec(
		"run kexec",
		"executing kexec into NixOS installer",
		"kexec failed",
		kexecCmd,
		executioner.OnDryRun(func() {}),
	)
	if err != nil {
		return errors.Wrap(err, "kexec failed")
	}

	return nil
}

// waitForKexecReboot waits for the machine to disconnect and reconnect after kexec.
func waitForKexecReboot(exc *executioner.Executioner, machineI *machine.Machine) error {
	activeSSH := machineI.GetActiveSSH()

	err := executioner.WaitForDisconnect(exc, activeSSH, "waiting for machine to become unreachable")
	if err != nil {
		return errors.Wrap(err, "wait for disconnect failed")
	}

	// After kexec, use the kexec SSH config (same hostname)
	machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeKexec })
	activeSSH = machineI.GetActiveSSH()

	err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to reconnect after kexec", "machine did not reconnect after kexec")
	if err != nil {
		return errors.Wrap(err, "wait for reconnect failed")
	}

	return verifyInstaller(exc)
}

func verifyInstaller(exc *executioner.Executioner) error {
	err := exc.Exec(
		"verify installer",
		"verifying NixOS installer",
		"not in NixOS installer",
		[]string{"cat", "/etc/os-release"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			output := log.Output.String()

			osRelease, err := osrelease.ReadString(output)
			if err != nil {
				return errors.Wrap(err, "error parsing /etc/os-release")
			}

			if osRelease["ID"] != "nixos" || osRelease["VARIANT_ID"] != "installer" {
				return ErrKexecBootFailed
			}

			return nil
		}),
		executioner.OnDryRun(func() {}),
	)

	return errors.Wrap(err, "failed to verify installer")
}
