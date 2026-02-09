package workflow

import (
	"fmt"
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

var KexecSupportedPlatforms = []string{"x86_64", "aarch64"}

func (w *Workflow) executeInspectPhaseMachine(machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Inspect, machine,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			mms := machine.MetaStatus

			commands := []struct {
				name        string
				runningMsg  string
				failMsg     string
				args        []string
				opts        []executioner.ExecOption
				skipOnError bool
			}{
				{
					name:       "TCP check",
					runningMsg: "checking TCP connectivity",
					failMsg:    "unreachable",
					args:       []string{"nc", "-zvw1", machine.SSH.Hostname, fmt.Sprintf("%d", machine.SSH.Port)},
					opts: []executioner.ExecOption{
						executioner.SkipIfLocal(),
						executioner.DisableAutoSshCommand(),
						executioner.OnSuccess(func(log *logs_command.CommandLog) error {
							mms.Reachable.Store(true)
							return nil
						}),
						executioner.OnDryRun(func() {
							mms.Reachable.Store(true)
						}),
					},
				},
				{
					name:       "SSH connect",
					runningMsg: "connecting via SSH",
					failMsg:    "SSH auth failed",
					args:       []string{"echo", "OK"},
					opts: []executioner.ExecOption{
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
					},
				},
				{
					name:       "architecture",
					runningMsg: "detecting architecture",
					failMsg:    "uname failed",
					args:       []string{"uname", "-m"},
					opts: []executioner.ExecOption{
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
					},
				},
				{
					name:       "superuser check",
					runningMsg: "checking superuser privileges",
					failMsg:    "checking superuser failed",
					args:       []string{"id", "-u"},
					opts: []executioner.ExecOption{
						executioner.OnFailure(func(log *logs_command.CommandLog, err error) error {
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
					},
				},
			}

			for _, cmd := range commands {
				err := exc.Exec(cmd.name, cmd.runningMsg, cmd.failMsg, cmd.args, cmd.opts...)
				if err != nil && !cmd.skipOnError {
					return err
				}
			}

			requiresKexec := false
			err := exc.Exec(
				"bootstrap detection",
				"detecting bootstrap status",
				"reading /etc/os-release failed",
				[]string{"cat", "/etc/os-release"},
				executioner.OnFailure(func(log *logs_command.CommandLog, err error) error {
					return errors.Wrap(err, log.String())
				}),
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					output := log.String()
					osrelease, err := osrelease.ReadString(output)
					if err != nil {
						return errors.Wrap(err, "error parsing /etc/os-release")
					}
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

			if !mms.Bootstrapped.Load() {
				if requiresKexec {
					return fmt.Errorf("kexec not implemented")
				}
				if machine.HardwareConfigPath != "" {
					err := exc.Exec(
						"generate config",
						"generating hardware config",
						"nixos-generate-config failed",
						append(machine.MaybeSudo(), "nixos-generate-config", "--show-hardware-config", "--no-filesystems", ">", machine.HardwareConfigPath),
					)
					if err != nil {
						return err
					}
				}
				return nil
			}

			genResult := exc.Exec(
				"get generation",
				"reading generation info",
				"failed to read generation symlink",
				[]string{"readlink", "/nix/var/nix/profiles/system"},
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					link := strings.TrimSpace(log.String())
					if strings.HasPrefix(link, "system-") && strings.HasSuffix(link, "-link") {
						genStr := strings.TrimSuffix(strings.TrimPrefix(link, "system-"), "-link")
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
			if genResult != nil {
				return genResult
			}

			dateResult := exc.Exec(
				"get generation date",
				"reading generation date",
				"failed to stat system profile",
				[]string{"stat", "-c", "%y", "/nix/var/nix/profiles/system"},
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					date := strings.TrimSpace(log.String())
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
			if dateResult != nil {
				return dateResult
			}

			return exc.Exec(
				"get kernel version",
				"reading kernel version",
				"uname failed",
				[]string{"uname", "-r"},
				executioner.OnSuccess(func(log *logs_command.CommandLog) error {
					mms.Kernel.Store(strings.TrimSpace(log.String()))
					return nil
				}),
				executioner.OnDryRun(func() {
					mms.Kernel.Store("DRY_RUN")
				}),
			)
		})
}
