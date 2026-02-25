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
	"github.com/pkg/errors"
)

const KexecURL = "https://github.com/nix-community/nixos-images/releases/latest/download/nixos-kexec-installer-noninteractive-<arch>-linux.tar.gz"

var KexecSupportedPlatforms = []string{"x86_64", "aarch64"}

func (w *Workflow) executeKexec(exc *executioner.Executioner, machine *config.Machine) error {
	arch := machine.MetaInspect.Architecture.Load()
	if arch == "" {
		return fmt.Errorf("architecture not detected, cannot determine kexec URL")
	}

	kexecURL := machine.Bootstrap.KexecURL
	if kexecURL == "" {
		if !slices.Contains(KexecSupportedPlatforms, arch) {
			return fmt.Errorf("arch %s is not supported by default kexec, supported are %s", strconv.Quote(arch), KexecSupportedPlatforms)
		}

		kexecURL = KexecURL
	}

	kexecURL = strings.ReplaceAll(kexecURL, "<arch>", arch)

	maybeSudo := machine.MaybeSudo()

	err := exc.Exec(
		"create kexec directory",
		"creating kexec directory",
		"failed to create kexec directory",
		append(append([]string{}, maybeSudo...), "mkdir", "-p", "$HOME/kexec"),
	)
	if err != nil {
		return err
	}

	if isURL(kexecURL) {
		err = exc.Exec(
			"download kexec tarball",
			"downloading kexec tarball",
			"failed to download kexec tarball",
			[]string{"curl", "--fail", "-#", "-L", "-o", "$HOME/kexec/kexec.tar", kexecURL},
		)
	} else {
		err = w.transferPlainFileOrDir(exc, machine, &config_attributes.PlainFileOrDirToTransfer{
			LocalPath:  kexecURL,
			RemotePath: "$HOME/kexec/kexec.tar",
		}, "kexec tarball")
	}
	if err != nil {
		return err
	}

	var tarArgs []string
	switch {
	case strings.HasSuffix(kexecURL, ".tar.gz") || strings.HasSuffix(kexecURL, ".tgz"):
		tarArgs = []string{"-xvzf", "$HOME/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.xz"):
		tarArgs = []string{"-xvJf", "$HOME/kexec/kexec.tar"}
	case strings.HasSuffix(kexecURL, ".tar.zst"):
		tarArgs = []string{"--use-compress-program=zstd", "-xvf", "$HOME/kexec/kexec.tar"}
	default:
		tarArgs = []string{"-xvf", "$HOME/kexec/kexec.tar"}
	}
	tarArgs = append(tarArgs, "-C", "$HOME/kexec")

	err = exc.Exec(
		"extract kexec tarball",
		"extracting kexec tarball",
		"failed to extract kexec tarball",
		append(append([]string{}, maybeSudo...), append([]string{"tar"}, tarArgs...)...),
	)
	if err != nil {
		return err
	}

	kexecExtraFlags := machine.Bootstrap.KexecExtraFlags
	var kexecCmd []string
	if kexecExtraFlags != "" {
		escapedFlags := strings.ReplaceAll(kexecExtraFlags, "'", "'\\''")
		kexecCmd = []string{"sh", "-c", fmt.Sprintf("$HOME/kexec/kexec/run --kexec-extra-flags '%s'", escapedFlags)}
	} else {
		kexecCmd = []string{"sh", "-c", "$HOME/kexec/kexec/run"}
	}

	if len(maybeSudo) > 0 {
		kexecCmd = append(append([]string{}, maybeSudo...), kexecCmd...)
	}

	err = exc.Exec(
		"run kexec",
		"executing kexec into NixOS installer",
		"kexec failed",
		kexecCmd,
		executioner.OnSuccess(func(log *logs_command.CommandLog) error {
			if err := w.waitForKexecReconnect(exc, machine); err != nil {
				return err
			}
			return w.verifyInstaller(exc)
		}),
		executioner.OnDryRun(func() {}),
	)

	return err
}

func (w *Workflow) waitForKexecReconnect(exc *executioner.Executioner, machine *config.Machine) error {
	hostname := machine.SSH.Hostname
	port := machine.SSH.Port

	waitForDisconnectScript := fmt.Sprintf(
		`for i in $(seq 1 300); do if ! nc -zvw1 %s %d 2>/dev/null; then exit 0; fi; sleep 1; done; exit 1`,
		hostname, port,
	)

	err := exc.Exec(
		"wait for disconnect",
		"waiting for machine to become unreachable",
		"failed to wait for disconnect",
		[]string{"sh", "-c", waitForDisconnectScript},
		executioner.DisableAutoSshCommand(),
	)
	if err != nil {
		return err
	}

	reconnectScript := fmt.Sprintf(
		`for i in $(seq 1 60); do if nc -zvw1 %s %d 2>/dev/null; then exit 0; fi; sleep 5; done; exit 1`,
		hostname, port,
	)

	return exc.Exec(
		"wait for reconnect",
		"waiting for machine to reconnect after kexec",
		"machine did not reconnect after kexec",
		[]string{"sh", "-c", reconnectScript},
		executioner.DisableAutoSshCommand(),
	)
}

func (w *Workflow) verifyInstaller(exc *executioner.Executioner) error {
	return exc.Exec(
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
}

// Helpers

func isURL(s string) bool {
	_, err := url.Parse(s)
	return err == nil
}
