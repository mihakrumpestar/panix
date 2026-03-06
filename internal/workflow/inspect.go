package workflow

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeInspectPhaseMachine(machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Inspect, machine,
		func(exc *executioner.Executioner, phaseLog *phase.PhaseLog) error {
			mms := machine.MetaInspect

			if machine.SSH.IsLocal {
				machine.SwitchToRegularSSH()
				mms.Reachable.Store(true)
				mms.SSHConnectable.Store(true)
			} else {
				err := checkSSHReachability(exc, machine, mms)
				if err != nil {
					return err
				}
			}

			err := checkSSHConnection(exc, mms)
			if err != nil {
				return err
			}

			err = detectArchitecture(exc, mms)
			if err != nil {
				return err
			}

			err = checkSuperuser(exc, mms)
			if err != nil {
				return err
			}

			err = detectBootstrapStatus(exc, machine, mms)
			if err != nil {
				return err
			}

			if !mms.Bootstrapped.Load() {
				return handleUnbootstrapped(exc, machine)
			}

			err = readGenerationInfo(exc, mms)
			if err != nil {
				return err
			}

			err = readGenerationDate(exc, mms)
			if err != nil {
				return err
			}

			return readKernelVersion(exc, mms)
		})
}

func checkSSHReachability(exc *executioner.Executioner, machine *config.Machine, mms *config.MetaInspect) error {
	err := exc.ExecFn(
		"SSH reachability check",
		"checking SSH reachability",
		"SSH unreachable",
		func() error {
			bootstrapSSHReachable := false
			regularSSHReachable := false

			if machine.Bootstrap.SSH != nil {
				bootstrapSSHReachable = machine.Bootstrap.SSH.ReachabilityCheck(SSHReachabilityCheckTimeout)
			}

			if bootstrapSSHReachable {
				machine.SwitchToBootstrapSSH()
			} else {
				regularSSHReachable = machine.SSH.ReachabilityCheck(SSHReachabilityCheckTimeout)
				if regularSSHReachable {
					machine.SwitchToRegularSSH()
				}
			}

			if !bootstrapSSHReachable && !regularSSHReachable {
				return fmt.Errorf("both bootstrap SSH and regular SSH are unreachable")
			}

			mms.Reachable.Store(true)

			return nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "reachability check failed")
	}

	return nil
}

func checkSSHConnection(exc *executioner.Executioner, mms *config.MetaInspect) error {
	err := exc.Exec(
		"SSH connect",
		"connecting via SSH",
		"SSH auth failed",
		[]string{"echo", "OK"},
		executioner.SkipIfLocal(),
		executioner.OnFailure(func(log *command.CommandLog, err error) error {
			return errors.Wrap(err, log.String())
		}),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			mms.SSHConnectable.Store(true)

			return nil
		}),
		executioner.OnDryRun(func() {
			mms.SSHConnectable.Store(true)
		}),
	)
	if err != nil {
		return errors.Wrap(err, "SSH connect failed")
	}

	return nil
}

func detectArchitecture(exc *executioner.Executioner, mms *config.MetaInspect) error {
	err := exc.Exec(
		"architecture",
		"detecting architecture",
		"uname failed",
		[]string{"uname", "-m"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
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
		return errors.Wrap(err, "architecture detection failed")
	}

	return nil
}

func checkSuperuser(exc *executioner.Executioner, mms *config.MetaInspect) error {
	err := exc.Exec(
		"superuser check",
		"checking superuser privileges",
		"checking superuser failed",
		[]string{"id", "-u"},
		executioner.OnFailure(func(log *command.CommandLog, err error) error {
			return errors.Wrap(err, log.String())
		}),
		executioner.OnSuccess(func(log *command.CommandLog) error {
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
		return errors.Wrap(err, "superuser check failed")
	}

	return nil
}

func detectBootstrapStatus(exc *executioner.Executioner, machine *config.Machine, mms *config.MetaInspect) error {
	err := exc.Exec(
		"bootstrap detection",
		"detecting bootstrap status",
		"bootstrap detection failed",
		[]string{"cat", "/etc/os-release"},
		executioner.OnFailure(func(log *command.CommandLog, err error) error {
			return errors.Wrap(err, log.String())
		}),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			output := log.String()

			osrelease, err := osrelease.ReadString(output)
			if err != nil {
				return errors.Wrap(err, "error parsing /etc/os-release")
			}

			if osrelease["ID"] == "nixos" && osrelease["VARIANT_ID"] == "installer" {
				mms.Bootstrapped.Store(false)

				return nil
			}

			if osrelease["ID"] != "nixos" {
				mms.RequiresKexec.Store(true)
				mms.Bootstrapped.Store(false)

				if machine.Flags.Bootstrap.DisableAuto {
					return fmt.Errorf("machine requires kexec but automatic bootstrap is disabled")
				}

				return nil
			}

			mms.Bootstrapped.Store(true)
			mms.Nixos.Store(osrelease["BUILD_ID"])

			return nil
		}),
		executioner.OnDryRun(func() {
			mms.Bootstrapped.Store(false)
			mms.RequiresKexec.Store(true)
		}),
	)
	if err != nil {
		return errors.Wrap(err, "bootstrap detection failed")
	}

	return nil
}

func handleUnbootstrapped(exc *executioner.Executioner, machine *config.Machine) error {
	if machine.HardwareConfigPath == "" {
		return nil
	}

	err := exc.Exec(
		"generate config",
		"generating hardware config",
		"nixos-generate-config failed",
		append(machine.MaybeSudo(), "nixos-generate-config", "--show-hardware-config", "--no-filesystems", ">", machine.HardwareConfigPath),
	)
	if err != nil {
		return errors.Wrap(err, "hardware config generation failed")
	}

	return nil
}

func readGenerationInfo(exc *executioner.Executioner, mms *config.MetaInspect) error {
	err := exc.Exec(
		"get generation",
		"reading generation info",
		"failed to read generation symlink",
		[]string{"readlink", "/nix/var/nix/profiles/system"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
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
	if err != nil {
		return errors.Wrap(err, "failed to get generation")
	}

	return nil
}

func readGenerationDate(exc *executioner.Executioner, mms *config.MetaInspect) error {
	err := exc.Exec(
		"get generation date",
		"reading generation date",
		"failed to stat system profile",
		[]string{"stat", "-c", "%y", "/nix/var/nix/profiles/system"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
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
	if err != nil {
		return errors.Wrap(err, "failed to get generation date")
	}

	return nil
}

func readKernelVersion(exc *executioner.Executioner, mms *config.MetaInspect) error {
	err := exc.Exec(
		"get kernel version",
		"reading kernel version",
		"uname failed",
		[]string{"uname", "-r"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			mms.Kernel.Store(strings.TrimSpace(log.String()))

			return nil
		}),
		executioner.OnDryRun(func() {
			mms.Kernel.Store("DRY_RUN")
		}),
	)
	if err != nil {
		return errors.Wrap(err, "failed to read kernel version")
	}

	return nil
}
