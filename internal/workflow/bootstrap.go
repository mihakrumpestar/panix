package workflow

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

var (
	ErrDiskoNoOutputPaths       = errors.New("disko build output did not contain any output paths")
	ErrArchitectureNotSupported = errors.New("architecture not supported by default kexec")
	ErrKexecBootFailed          = errors.New("kexec did not boot into NixOS installer")
)

const KexecURL = "https://github.com/nix-community/nixos-images/releases/latest/download/nixos-kexec-installer-noninteractive-<arch>-linux.tar.gz"

var KexecSupportedPlatforms = []string{"x86_64", "aarch64"}

func (w *Workflow) executeBootstrapPhaseMachine(fleetLeaf *fleet.FleetLeaf) error {
	return w.Phase(phase.Bootstrap, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error {
			flake := fleetLeaf.Flake
			configuration := fleetLeaf.Configuration
			machine := fleetLeaf.Machine

			mi := machine.MetaInspect.Load()
			if mi != nil && mi.RequiresKexec {
				err := w.executeKexec(exc, machine)
				if err != nil {
					return err
				}
			}

			installables := []string{fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.diskoScript", flake.URL, configuration.Name)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, fleetLeaf, installables, "disko")
			if err != nil {
				return err
			}

			if len(parsedOutput) == 0 {
				return ErrDiskoNoOutputPaths
			}

			diskoScript := parsedOutput[0].Outputs.Out

			err = executeTransferPhaseMachineWrapper(exc, machine, []string{diskoScript}, false)
			if err != nil {
				return err
			}

			// Upload disk encryption keys BEFORE running disko
			// Keys must be available for LUKS unlocking during partitioning
			if len(machine.Bootstrap.DiskEncryptionKeys) > 0 {
				err = w.executeDiskEncryptionKeys(exc, machine)
				if err != nil {
					return err
				}
			}

			err = exc.Exec(
				"disko",
				"partitioning disk",
				"diskoScript failed",
				[]string{diskoScript},
			)
			if err != nil {
				return errors.Wrap(err, "disko failed")
			}

			return exc.ExecuteHooks(machine.Bootstrap.PostBootstrapHooks, "post bootstrap hook")
		},
	)
}

// executeDiskEncryptionKeys transfers disk encryption keys to the target machine.
// Must be called BEFORE disko runs, so keys are available for LUKS unlocking.
func (w *Workflow) executeDiskEncryptionKeys(
	exc *executioner.Executioner,
	machine *machine.Machine,
) error {
	for _, diskEncryptionKey := range machine.Bootstrap.DiskEncryptionKeys {
		err := w.transferPlainFileOrDir(exc, machine, diskEncryptionKey, "disk encryption key", false)
		if err != nil {
			return errors.Wrapf(err, "failed to transfer disk encryption key to %s", diskEncryptionKey.RemotePath)
		}
	}

	return nil
}

func (w *Workflow) executeKexec(exc *executioner.Executioner, machine *machine.Machine) error {
	mi := machine.MetaInspect.Load()
	arch := ""
	if mi != nil {
		arch = mi.Architecture
	}

	if arch == "" || arch == "DRY_RUN" {
		arch = "x86_64"
	}

	return w.executeKexecReal(exc, machine, arch)
}

// executeKexecReal performs the actual kexec bootstrap process.
func (w *Workflow) executeKexecReal(exc *executioner.Executioner, machineI *machine.Machine, arch string) error {
	kexecURL, err := resolveKexecURL(machineI, arch)
	if err != nil {
		return err
	}

	err = w.createKexecDirectory(exc, machineI)
	if err != nil {
		return err
	}

	err = w.downloadOrTransferKexec(exc, machineI, kexecURL)
	if err != nil {
		return err
	}

	err = w.extractKexecTarball(exc, machineI, kexecURL)
	if err != nil {
		return err
	}

	err = w.runKexecCommand(exc, machineI)
	if err != nil {
		return err
	}

	err = w.waitForKexecReboot(exc, machineI)
	if err != nil {
		return err
	}

	machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
		mi.RequiresKexec = false
	})

	return nil
}

// resolveKexecURL returns the kexec URL, using default if not configured.
func resolveKexecURL(machine *machine.Machine, arch string) (string, error) {
	kexecURL := ""
	if machine.Bootstrap.Kexec != nil {
		kexecURL = machine.Bootstrap.Kexec.URL
	}

	if kexecURL == "" {
		if !slices.Contains(KexecSupportedPlatforms, arch) {
			return "", errors.Wrapf(ErrArchitectureNotSupported, "%s (supported: %s)", strconv.Quote(arch), KexecSupportedPlatforms)
		}

		kexecURL = KexecURL
	}

	return strings.ReplaceAll(kexecURL, "<arch>", arch), nil
}

// createKexecDirectory creates the temporary directory for kexec files.
func (w *Workflow) createKexecDirectory(exc *executioner.Executioner, machine *machine.Machine) error {
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
func (w *Workflow) downloadOrTransferKexec(exc *executioner.Executioner, machine *machine.Machine, kexecURL string) error {
	var err error
	if isURL(kexecURL) {
		err = exc.Exec(
			"download kexec tarball",
			"downloading kexec tarball",
			"failed to download kexec tarball",
			[]string{"curl", "--fail", "-#", "-L", "-C", "-", "-o", "/tmp/kexec/kexec.tar", kexecURL},
		)
	} else {
		err = w.transferPlainFileOrDir(exc, machine, &attributes.PlainFileOrDirToTransfer{
			LocalPath:  kexecURL,
			RemotePath: "/tmp/kexec/kexec.tar",
		}, "kexec tarball", false)
	}

	return err
}

// extractKexecTarball extracts the kexec tarball to the temporary directory.
func (w *Workflow) extractKexecTarball(exc *executioner.Executioner, machine *machine.Machine, kexecURL string) error {
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
		return []string{"-xvzf", "/tmp/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.xz"):
		return []string{"-xvJf", "/tmp/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.zst"):
		return []string{"--use-compress-program=zstd", "-xvf", "/tmp/kexec/kexec.tar"}
	default:
		return []string{"-xvf", "/tmp/kexec/kexec.tar"}
	}
}

// runKexecCommand executes the kexec script to boot into the NixOS installer.
func (w *Workflow) runKexecCommand(exc *executioner.Executioner, machine *machine.Machine) error {
	kexecCmd := append(machine.MaybeSudo(), []string{"/tmp/kexec/kexec/run"}...)

	if machine.Bootstrap.Kexec != nil && machine.Bootstrap.Kexec.ExtraFlags != "" {
		kexecCmd = append(kexecCmd, "--kexec-extra-flags", machine.Bootstrap.Kexec.ExtraFlags)
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
func (w *Workflow) waitForKexecReboot(exc *executioner.Executioner, machineI *machine.Machine) error {
	activeSSH := machineI.GetActiveSSH()

	err := executioner.WaitForDisconnect(exc, activeSSH, "waiting for machine to become unreachable")
	if err != nil {
		return errors.Wrap(err, "wait for disconnect failed")
	}

	// After kexec, use the kexec SSH config (default: same hostname, port 22)
	machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeKexec })
	activeSSH = machineI.GetActiveSSH()

	err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to reconnect after kexec", "machine did not reconnect after kexec")
	if err != nil {
		return errors.Wrap(err, "wait for reconnect failed")
	}

	return w.verifyInstaller(exc)
}

func (w *Workflow) verifyInstaller(exc *executioner.Executioner) error {
	err := exc.Exec(
		"verify installer",
		"verifying NixOS installer",
		"not in NixOS installer",
		[]string{"cat", "/etc/os-release"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			output := log.String()

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

// Helpers.

func isURL(s string) bool {
	parsedURL, err := url.Parse(s)
	if err != nil {
		return false
	}

	// Ensure it has a valid scheme (http or https) and a host
	return (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != ""
}
