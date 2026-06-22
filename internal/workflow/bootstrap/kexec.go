package bootstrap

import (
	"net/url"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/osrelease"
	"github.com/pkg/errors"
)

var ErrKexecBootFailed = errors.New("kexec did not boot into NixOS installer")

func executeKexec(exc *executioner.Executioner, machineI *machine.Machine) error {
	arch := machineI.MetaInspect.Load().Architecture.String()

	if arch == "DRY_RUN" {
		arch = "x86_64"
	}

	kexecURL := strings.ReplaceAll(machineI.Bootstrap.Kexec.Image.String(), "<arch>", arch)

	err := createKexecDirectory(exc, machineI)
	if err != nil {
		return err
	}

	err = downloadOrTransferKexec(exc, machineI, kexecURL)
	if err != nil {
		return err
	}

	err = extractKexecTarball(exc, machineI, kexecURL)
	if err != nil {
		return err
	}

	err = runKexecCommand(exc, machineI)
	if err != nil {
		return err
	}

	err = waitForKexecReboot(exc, machineI)
	if err != nil {
		return err
	}

	machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
		mi.RequiresKexec = false
	})

	return nil
}

// createKexecDirectory creates the temporary directory for kexec files.
func createKexecDirectory(exc *executioner.Executioner, machine *machine.Machine) error {
	err := exc.Exec(
		"create kexec directory",
		"creating kexec directory",
		"failed to create kexec directory",
		append(machine.MaybeSudo(), "mkdir", "-p", "/tmp/kexec"),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create kexec directory")
	}

	return nil
}

// downloadOrTransferKexec downloads the kexec tarball from URL or transfers it from local path.
func downloadOrTransferKexec(exc *executioner.Executioner, machine *machine.Machine, kexecURL string) error {
	var err error
	if isURL(kexecURL) {
		err = exc.Exec(
			"download kexec tarball",
			"downloading kexec tarball",
			"failed to download kexec tarball",
			[]string{"curl", "--fail", "-#", "-L", "-C", "-", "-o", "/tmp/kexec/kexec.tar", kexecURL},
		)
	} else {
		err = phaseops.TransferFile(exc, machine, attributes.PlainFileOrDirToTransfer{
			LocalPath:  kexecURL,
			RemotePath: "/tmp/kexec/kexec.tar",
		}, "kexec tarball", false)
	}

	return errors.Wrap(err, "kexec tarball transfer failed")
}

// extractKexecTarball extracts the kexec tarball to the temporary directory.
func extractKexecTarball(exc *executioner.Executioner, machine *machine.Machine, kexecURL string) error {
	tarArgs := getTarArgs(kexecURL)
	tarArgs = append(tarArgs, "-C", "/tmp/kexec")

	err := exc.Exec(
		"extract kexec tarball",
		"extracting kexec tarball",
		"failed to extract kexec tarball",
		append(append(machine.MaybeSudo(), "tar"), tarArgs...),
	)
	if err != nil {
		return errors.Wrap(err, "failed to extract kexec tarball")
	}

	return nil
}

// getTarArgs returns the appropriate tar extraction arguments based on file extension.
func getTarArgs(kexecURL string) []string {
	switch {
	case strings.HasSuffix(kexecURL, ".tar.gz") || strings.HasSuffix(kexecURL, ".tgz"):
		return []string{"-xzf", "/tmp/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.xz"):
		return []string{"-xJf", "/tmp/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.zst"):
		return []string{"--use-compress-program=zstd", "-xf", "/tmp/kexec/kexec.tar"}
	default:
		return []string{"-xf", "/tmp/kexec/kexec.tar"}
	}
}

// runKexecCommand executes the kexec script to boot into the NixOS installer.
func runKexecCommand(exc *executioner.Executioner, machine *machine.Machine) error {
	kexecCmd := append(machine.MaybeSudo(), []string{"/tmp/kexec/kexec/run"}...)

	if len(machine.Bootstrap.Kexec.ExtraFlags) != 0 {
		extraFlags := append([]string{"--kexec-extra-flags"}, machine.Bootstrap.Kexec.ExtraFlags...)

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

// Helpers

func isURL(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	return (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != ""
}
