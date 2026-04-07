package workflow

import (
	"slices"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var (
	ErrArchitectureOutputEmpty = errors.New("architecture output was empty")
	ErrPlatformUnsupported     = errors.New("platform unsupported, kexec supports limited platforms")
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

			err = validateSSHMachineState(exc, machine, mms)
			if err != nil {
				return err
			}

			if !mms.Bootstrapped.Load() {
				return handleUnbootstrapped(exc, machine)
			}

			return readGenerations(exc, mms)
		})
}

func checkSSHReachability(exc *executioner.Executioner, machine *config.Machine, mms *config.MetaInspect) error {
	err := exc.ExecFn(
		"SSH reachability check",
		"checking SSH reachability",
		"SSH unreachable",
		func(_ *command.CommandLog) error {
			if machine.Bootstrap.SSH != nil {
				bootstrapSSHReachable := machine.Bootstrap.SSH.ReachabilityCheck(SSHReachabilityCheckTimeout)

				if !bootstrapSSHReachable {
					return errors.New("bootstrap SSH is configured but unreachable")
				}

				machine.SwitchToBootstrapSSH()
			} else {
				regularSSHReachable := machine.SSH.ReachabilityCheck(SSHReachabilityCheckTimeout)

				if !regularSSHReachable {
					return errors.New("regular SSH is unreachable")
				}

				machine.SwitchToRegularSSH()
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
				return ErrArchitectureOutputEmpty
			}

			if !slices.Contains(KexecSupportedPlatforms, architecture) {
				return errors.Wrapf(ErrPlatformUnsupported, "%s (supported: %s)", strconv.Quote(architecture), KexecSupportedPlatforms)
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

				return nil
			}

			if machine.Bootstrap.ForceBootstrap {
				mms.Bootstrapped.Store(false)
				mms.RequiresKexec.Store(machine.Bootstrap.ForceBootstrapKexec)

				return nil
			}

			mms.Bootstrapped.Store(true)

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

func readGenerations(exc *executioner.Executioner, mms *config.MetaInspect) error {
	err := exc.Exec(
		"list generations",
		"listing generations",
		"failed to list generations",
		[]string{"nixos-rebuild", "list-generations"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			genData, err := parseGenerationsOutput(log.String())
			if err != nil {
				return err
			}

			mms.Generations.Store(genData)

			return nil
		}),
		executioner.OnDryRun(func() {
			generations, _ := omap.New[uint, *config.GenerationInfo]()

			_ = generations.Set(1, &config.GenerationInfo{Date: "DRY_RUN", Nixos: "DRY_RUN", Kernel: "DRY_RUN", Current: true})

			mms.Generations.Store(&config.GenerationsData{
				Current:     1,
				Generations: generations,
			})
		}),
	)

	return errors.Wrap(err, "failed to list generations")
}

func validateSSHMachineState(exc *executioner.Executioner, machine *config.Machine, mms *config.MetaInspect) error {
	isBootstrapped := mms.Bootstrapped.Load()

	err := exc.ExecFn(
		"validate SSH config upon detected machine state",
		"validating SSH config",
		"SSH config invalid for detected machine state",
		func(_ *command.CommandLog) error {
			if machine.Bootstrap.SSH != nil && isBootstrapped && !machine.Bootstrap.ForceBootstrap {
				return errors.New("bootstrap SSH is configured but machine is already bootstrapped; set \"force_bootstrap\" to re-bootstrap, or remove bootstrap SSH configuration") //nolint:lll
			}

			if !isBootstrapped && machine.Bootstrap.SSH == nil && !machine.Bootstrap.ForceBootstrap {
				return errors.New("bootstrapping requires bootstrap SSH to be configured (or set \"force_bootstrap\" to re-bootstrap with regular SSH)")
			}

			return nil
		},
	)

	return errors.Wrap(err, "SSH config validation failed upon detected machine state")
}

func parseGenerationsOutput(output string) (*config.GenerationsData, error) {
	lines := strings.Split(output, "\n")

	generations, err := omap.New[uint, *config.GenerationInfo]()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create generations map")
	}

	var currentGenNum uint

	for i, line := range lines {
		genNum, gen, ok := parseGenerationLine(i, line)
		if !ok {
			continue
		}

		if gen.Current {
			currentGenNum = genNum
		}

		err = generations.Set(genNum, gen)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set generation")
		}
	}

	if generations.Len() == 0 {
		return nil, errors.New("no generations found - ensure this is a NixOS system with system profiles")
	}

	return &config.GenerationsData{
		Current:     currentGenNum,
		Generations: generations,
	}, nil
}

func parseGenerationLine(idx int, line string) (uint, *config.GenerationInfo, bool) {
	const minFields = 5

	line = strings.TrimSpace(line)
	if line == "" || idx == 0 {
		return 0, nil, false
	}

	fields := strings.Fields(line)
	if len(fields) < minFields {
		return 0, nil, false
	}

	parsedNum, err := strconv.ParseUint(fields[0], 10, 0)
	if err != nil {
		return 0, nil, false
	}

	gen := &config.GenerationInfo{
		Date:   fields[1] + " " + fields[2],
		Nixos:  fields[3],
		Kernel: fields[4],
	}

	const currentFieldIdx = 6

	if len(fields) > currentFieldIdx && fields[len(fields)-1] == "True" {
		gen.Current = true
	}

	return uint(parsedNum), gen, true
}
