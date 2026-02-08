package workflow

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var (
	KexecSupportedPlatforms = []string{"x86_64", "aarch64"}
)

func (w *Workflow) executeInspectPhaseMachine(machine *config.Machine) (err error) {
	return w.Phase(machine.Attributes.Xpath, phases.Inspect, machine,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			mms := machine.MetaStatus

			// TCP check

			commandWithArgs := []string{"nc", "-zvw1", machine.SSH.Hostname, fmt.Sprintf("%d", machine.SSH.Port)}

			err = exc.Exec(
				"TCP check",
				"unreachable",
				commandWithArgs,
				executioner.SkipIfLocal(),
				executioner.DisableAutoSshCommand(),
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					mms.Reachable.Store(true)
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.Reachable.Store(true)
				}),
			)
			if err != nil {
				return err
			}

			// SSH connect

			commandWithArgs = []string{"echo", "OK"}

			err = exc.Exec(
				"SSH connect",
				"SSH auth failed",
				commandWithArgs,
				executioner.SkipIfLocal(),
				executioner.OnFailure(func(log *logs_command.CommandLog, err error) error {
					return errors.Wrap(err, log.String())
				}),
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					mms.SSHConnectable.Store(true)
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.SSHConnectable.Store(true)
				}),
			)
			if err != nil {
				return err
			}

			// Architecture

			commandWithArgs = []string{"uname", "-m"}

			err = exc.Exec(
				"architecture",
				"uname failed",
				commandWithArgs,
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					architecture := strings.Trim(log.String(), "\n")

					if architecture == "" {
						return fmt.Errorf("architecture output was empty")
					}

					if !slices.Contains(KexecSupportedPlatforms, architecture) {
						return fmt.Errorf("platform %s is unsupported, kexec currently only supports %s platforms", strconv.Quote(architecture), KexecSupportedPlatforms)
					}

					mms.Architecture.Store(architecture)
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.Architecture.Store("DRY_RUN")
				}),
			)
			if err != nil {
				return err
			}

			// IsSudo

			commandWithArgs = []string{"id", "-u"}

			err = exc.Exec(
				"superuser check",
				"checking superuser failed",
				commandWithArgs,
				executioner.OnFailure(
					func(log *logs_command.CommandLog, err error) error {
						return errors.Wrap(err, log.String())
					}),
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					output := strings.Trim(log.String(), "\n ")

					parsedOutput, err := strconv.ParseUint(output, 10, 64)
					if err != nil {
						return errors.Wrapf(err, "failed to parse raw output %s to uint", strconv.Quote(output))
					}

					mms.IsRoot.Store(parsedOutput == 0)
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.IsRoot.Store(true)
				}),
			)
			if err != nil {
				return err
			}

			// Run bootstrap detection and get OS info

			requiresKexec := false
			commandWithArgs = []string{"cat", "/etc/os-release"}

			err = exc.Exec(
				"bootstrap detection",
				"reading /etc/os-release failed",
				commandWithArgs,
				executioner.OnFailure(
					func(log *logs_command.CommandLog, err error) error {
						return errors.Wrap(err, log.String())
					}),
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					output := log.String()

					osrelease, err := osrelease.ReadString(output)
					if err != nil {
						return errors.Wrap(err, "error parsing /etc/os-release")
					}

					// Get NixOS version from os-release
					mms.Nixos.Store(osrelease["BUILD_ID"])

					if osrelease["ID"] == "nixos" && osrelease["VARIANT_ID"] == "installer" {
						mms.Bootstrapped.Store(false)
						return nil
					}

					if osrelease["ID"] != "nixos" {
						requiresKexec = true
						mms.Bootstrapped.Store(false)
						return nil
					}

					mms.Bootstrapped.Store(true)
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.Bootstrapped.Store(false)
				}),
			)
			if err != nil {
				return err
			}

			// Bootstrap part

			if !mms.Bootstrapped.Load() {

				// Kexec
				if requiresKexec {
					log.Fatal("TODO:")
					/*
						"x86_64 | aarch64)  kexecUrl="https://github.com/nix-community/nixos-images/releases/download/nixos-25.05/nixos-kexec-installer-noninteractive-${isArch}-linux.tar.gz"
						"TMPDIR=/root/kexec setsid --wait ${maybeSudo} /root/kexec/kexec/run"
						 ssh into kexec
					*/
				}

				// Generate-config
				if !mms.Bootstrapped.Load() && machine.HardwareConfigPath != "" {
					commandWithArgs := append(machine.MaybeSudo(), "nixos-generate-config", "--show-hardware-config", "--no-filesystems", ">", machine.HardwareConfigPath)

					err := exc.Exec(
						"generate config",
						"nixos-generate-config failed",
						commandWithArgs,
					)
					if err != nil {
						return err
					}
				}

				return nil
			}

			// Regular (non-bootstrap) part

			// Get current generation info using multiple small commands instead of
			// nixos-rebuild list-generations --json which returns all 400+ generations

			// Get generation number from symlink
			err = exc.Exec(
				"get generation",
				"failed to read generation symlink",
				[]string{"readlink", "/nix/var/nix/profiles/system"},
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					link := strings.TrimSpace(log.String())
					// Parse "system-447-link" -> 447
					if strings.HasPrefix(link, "system-") && strings.HasSuffix(link, "-link") {
						genStr := strings.TrimPrefix(link, "system-")
						genStr = strings.TrimSuffix(genStr, "-link")
						if gen, err := strconv.ParseUint(genStr, 10, 32); err == nil {
							mms.Generation.Store(uint32(gen))
						}
					}
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.Generation.Store(1)
				}),
			)
			if err != nil {
				return err
			}

			// Get date from profile using stat
			err = exc.Exec(
				"get generation date",
				"failed to stat system profile",
				[]string{"stat", "-c", "%y", "/nix/var/nix/profiles/system"},
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					date := strings.TrimSpace(log.String())
					// Remove nanoseconds: "2026-02-05 18:06:06.705587882 +0100" -> "2026-02-05 18:06:06"
					if idx := strings.Index(date, "."); idx != -1 {
						date = date[:idx]
					}
					mms.Date.Store(date)
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.Date.Store("DRY_RUN")
				}),
			)
			if err != nil {
				return err
			}

			// Get kernel version
			err = exc.Exec(
				"get kernel version",
				"uname failed",
				[]string{"uname", "-r"},
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					kernel := strings.TrimSpace(log.String())
					mms.Kernel.Store(kernel)
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.Kernel.Store("DRY_RUN")
				}),
			)
			if err != nil {
				return err
			}

			return nil
		})
}
