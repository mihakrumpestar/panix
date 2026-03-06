package workflow

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

const KexecURL = "https://github.com/nix-community/nixos-images/releases/latest/download/nixos-kexec-installer-noninteractive-<arch>-linux.tar.gz"

var KexecSupportedPlatforms = []string{"x86_64", "aarch64"}

func (w *Workflow) executeBootstrapPhaseMachine(flake *config.Flake, configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Bootstrap, machine,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			if machine.MetaInspect.RequiresKexec.Load() {
				if err := w.executeKexec(exc, machine); err != nil {
					return err
				}
			}

			installables := []string{fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.diskoScript", flake.URL, configuration.Name)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, phaseLog, flake, configuration, installables, "disko")
			if err != nil {
				return err
			}

			if len(parsedOutput) == 0 {
				return fmt.Errorf("disko build output did not contain any output paths")
			}

			diskoScript := parsedOutput[0].Outputs.Out

			err = executeTransferPhaseMachineWrapper(exc, phaseLog, machine, []string{diskoScript}, false)
			if err != nil {
				return err
			}

			// Upload disk encryption keys BEFORE running disko
			// Keys must be available for LUKS unlocking during partitioning
			if len(machine.Bootstrap.DiskEncryptionKeys) > 0 {
				if err := w.executeDiskEncryptionKeys(exc, machine, phaseLog); err != nil {
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

// executeDiskEncryptionKeys transfers disk encryption keys to the target machine
// Must be called BEFORE disko runs, so keys are available for LUKS unlocking
func (w *Workflow) executeDiskEncryptionKeys(
	exc *executioner.Executioner,
	machine *config.Machine,
	phaseLog *logs_phase.PhaseLog,
) error {
	for _, diskEncryptionKey := range machine.Bootstrap.DiskEncryptionKeys {

		err := w.transferPlainFileOrDir(exc, machine, diskEncryptionKey, "disk encryption key", false)
		if err != nil {
			return errors.Wrapf(err, "failed to transfer disk encryption key to %s", diskEncryptionKey.RemotePath)
		}
	}

	return nil
}

func (w *Workflow) executeKexec(exc *executioner.Executioner, machine *config.Machine) error {
	arch := machine.MetaInspect.Architecture.Load()
	if arch == "" || arch == "DRY_RUN" {
		arch = "x86_64"
	}
	return w.executeKexecReal(exc, machine, arch)
}

func (w *Workflow) executeKexecReal(exc *executioner.Executioner, machine *config.Machine, arch string) error {
	kexecURL := machine.Bootstrap.KexecURL
	if kexecURL == "" {
		if !slices.Contains(KexecSupportedPlatforms, arch) {
			return fmt.Errorf("arch %s is not supported by default kexec, supported are %s", strconv.Quote(arch), KexecSupportedPlatforms)
		}

		kexecURL = KexecURL
	}

	kexecURL = strings.ReplaceAll(kexecURL, "<arch>", arch)

	err := exc.Exec(
		"create kexec directory",
		"creating kexec directory",
		"failed to create kexec directory",
		append(machine.MaybeSudo(), "mkdir", "-p", "/tmp/kexec"),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create kexec directory")
	}

	if isURL(kexecURL) {
		err = exc.Exec(
			"download kexec tarball",
			"downloading kexec tarball",
			"failed to download kexec tarball",
			[]string{"curl", "--fail", "-#", "-L", "-C", "-", "-o", "/tmp/kexec/kexec.tar", kexecURL},
		)
	} else {
		err = w.transferPlainFileOrDir(exc, machine, &config_attributes.PlainFileOrDirToTransfer{
			LocalPath:  kexecURL,
			RemotePath: "/tmp/kexec/kexec.tar",
		}, "kexec tarball", false)
	}

	if err != nil {
		return err
	}

	var tarArgs []string

	switch {
	case strings.HasSuffix(kexecURL, ".tar.gz") || strings.HasSuffix(kexecURL, ".tgz"):
		tarArgs = []string{"-xvzf", "/tmp/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.xz"):
		tarArgs = []string{"-xvJf", "/tmp/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.zst"):
		tarArgs = []string{"--use-compress-program=zstd", "-xvf", "/tmp/kexec/kexec.tar"}
	default:
		tarArgs = []string{"-xvf", "/tmp/kexec/kexec.tar"}
	}

	tarArgs = append(tarArgs, "-C", "/tmp/kexec")

	err = exc.Exec(
		"extract kexec tarball",
		"extracting kexec tarball",
		"failed to extract kexec tarball",
		append(append(machine.MaybeSudo(), "tar"), tarArgs...),
	)
	if err != nil {
		return errors.Wrap(err, "failed to extract kexec tarball")
	}

	kexecCmd := append(machine.MaybeSudo(), []string{"/tmp/kexec/kexec/run"}...)

	kexecExtraFlags := machine.Bootstrap.KexecExtraFlags
	if kexecExtraFlags != "" {
		kexecCmd = append(kexecCmd, "--kexec-extra-flags", kexecExtraFlags)
	}

	err = exc.Exec(
		"run kexec",
		"executing kexec into NixOS installer",
		"kexec failed",
		kexecCmd,
		executioner.OnDryRun(func() {}),
	)
	if err != nil {
		return errors.Wrap(err, "kexec failed")
	}

	activeSSH := machine.MetaInspect.GetActiveSSH()
	err = executioner.WaitForDisconnect(exc, activeSSH, "waiting for machine to become unreachable")

	if err != nil {
		return errors.Wrap(err, "wait for disconnect failed")
	}

	err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to reconnect after kexec", "machine did not reconnect after kexec")
	if err != nil {
		return errors.Wrap(err, "wait for reconnect failed")
	}

	err = w.verifyInstaller(exc)
	if err != nil {
		return err
	}

	machine.MetaInspect.RequiresKexec.Store(false)
	return nil
}

func (w *Workflow) verifyInstaller(exc *executioner.Executioner) error {
	err := exc.Exec(
		"verify installer",
		"verifying NixOS installer",
		"not in NixOS installer",
		[]string{"cat", "/etc/os-release"},
		executioner.OnSuccess(func(log *logs_command.CommandLog) error {
			output := log.String()

			osRelease, err := osrelease.ReadString(output)
			if err != nil {
				return errors.Wrap(err, "error parsing /etc/os-release")
			}

			if osRelease["ID"] != "nixos" || osRelease["VARIANT_ID"] != "installer" {
				return fmt.Errorf("kexec did not boot into NixOS installer")
			}

			return nil
		}),
		executioner.OnDryRun(func() {}),
	)

	return errors.Wrap(err, "failed to verify installer")
}

// Helpers

func isURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	// Ensure it has a valid scheme (http or https) and a host
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
