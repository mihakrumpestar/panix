package workflow

import (
	"slices"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

var (
	ErrArchitectureOutputEmpty = errors.New("architecture output was empty")
	ErrPlatformUnsupported     = errors.New("platform unsupported, kexec supports limited platforms")
)

func (w *Workflow) executeInspectPhaseMachine(fleetLeaf *fleet.FleetLeaf) error {
	return w.Phase(phase.Inspect, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phaselogs.PhaseLog) error {
			machineI := fleetLeaf.Machine

			if machineI.SSH.IsLocal {
				machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeRegular })
				machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
					mi.Reachable = true
					mi.SSHConnectable = true
				})
			} else {
				err := checkSSHReachability(exc, machineI)
				if err != nil {
					return err
				}
			}

			err := checkSSHConnection(exc, machineI)
			if err != nil {
				return err
			}

			err = detectArchitecture(exc, machineI)
			if err != nil {
				return err
			}

			err = checkSuperuser(exc, machineI)
			if err != nil {
				return err
			}

			err = detectBootstrapStatus(exc, machineI)
			if err != nil {
				return err
			}

			err = validateSSHMachineState(exc, machineI)
			if err != nil {
				return err
			}

			mi := machineI.MetaInspect.Load()
			if mi != nil && !mi.Bootstrapped {
				return handleUnbootstrapped(exc, machineI)
			}

			return readGenerations(exc, machineI)
		})
}

func checkSSHReachability(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.ExecFn(
		"SSH reachability check",
		"checking SSH reachability",
		"SSH unreachable",
		func(_ *command.CommandLog) error {
			if machineI.Bootstrap.SSH != nil {
				bootstrapSSHReachable := machineI.Bootstrap.SSH.ReachabilityCheck(SSHReachabilityCheckTimeout)

				if !bootstrapSSHReachable {
					return errors.New("bootstrap SSH is configured but unreachable")
				}

				machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeBootstrap })
			} else {
				regularSSHReachable := machineI.SSH.ReachabilityCheck(SSHReachabilityCheckTimeout)

				if !regularSSHReachable {
					return errors.New("regular SSH is unreachable")
				}

				machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeRegular })
			}

			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Reachable = true
			})

			return nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "reachability check failed")
	}

	return nil
}

func checkSSHConnection(exc *executioner.Executioner, machineI *machine.Machine) error {
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
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.SSHConnectable = true
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.SSHConnectable = true
			})
		}),
	)
	if err != nil {
		return errors.Wrap(err, "SSH connect failed")
	}

	return nil
}

func detectArchitecture(exc *executioner.Executioner, machineI *machine.Machine) error {
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

			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Architecture = architecture
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Architecture = "DRY_RUN"
			})
		}),
	)
	if err != nil {
		return errors.Wrap(err, "architecture detection failed")
	}

	return nil
}

func checkSuperuser(exc *executioner.Executioner, machineI *machine.Machine) error {
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

			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.IsRoot = parsedOutput == 0
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.IsRoot = true
			})
		}),
	)
	if err != nil {
		return errors.Wrap(err, "superuser check failed")
	}

	return nil
}

func detectBootstrapStatus(exc *executioner.Executioner, machineI *machine.Machine) error {
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
				machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
					mi.Bootstrapped = false
					mi.Nixos = osrelease["VERSION"]
				})

				return nil
			}

			if osrelease["ID"] != "nixos" {
				machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
					mi.RequiresKexec = true
					mi.Bootstrapped = false
					mi.Nixos = osrelease["VERSION"]
				})

				return nil
			}

			if machineI.Bootstrap.ForceBootstrap {
				machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
					mi.Bootstrapped = false
					mi.RequiresKexec = machineI.Bootstrap.ForceBootstrapKexec
					mi.Nixos = osrelease["VERSION"]
				})

				return nil
			}

			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Bootstrapped = true
				mi.Nixos = osrelease["VERSION"]
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Bootstrapped = false
				mi.RequiresKexec = true
				mi.Nixos = "DRY_RUN"
			})
		}),
	)
	if err != nil {
		return errors.Wrap(err, "bootstrap detection failed")
	}

	return nil
}

func handleUnbootstrapped(exc *executioner.Executioner, machineI *machine.Machine) error {
	if machineI.HardwareConfigPath == "" {
		return nil
	}

	err := exc.Exec(
		"generate config",
		"generating hardware config",
		"nixos-generate-config failed",
		append(machineI.MaybeSudo(), "nixos-generate-config", "--show-hardware-config", "--no-filesystems", ">", machineI.HardwareConfigPath),
	)
	if err != nil {
		return errors.Wrap(err, "hardware config generation failed")
	}

	return nil
}

func readGenerations(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.Exec(
		"list generations",
		"listing generations",
		"failed to list generations",
		[]string{"nixos-rebuild", "list-generations"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			genData, currentGenInfo, err := parseGenerationsOutput(log.String())
			if err != nil {
				return err
			}

			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Generations = genData
				if currentGenInfo.Date != "" {
					mi.Date = currentGenInfo.Date
				}

				if currentGenInfo.Nixos != "" {
					mi.Nixos = currentGenInfo.Nixos
				}

				if currentGenInfo.Kernel != "" {
					mi.Kernel = currentGenInfo.Kernel
				}
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Generations = &machine.Generations{
					Current:   1,
					Available: []uint{1},
				}
				mi.Date = "DRY_RUN"
				mi.Nixos = "DRY_RUN"
				mi.Kernel = "DRY_RUN"
			})
		}),
	)

	return errors.Wrap(err, "failed to list generations")
}

func validateSSHMachineState(exc *executioner.Executioner, machineI *machine.Machine) error {
	mi := machineI.MetaInspect.Load()
	isBootstrapped := mi != nil && mi.Bootstrapped

	err := exc.ExecFn(
		"validate SSH config upon detected machine state",
		"validating SSH config",
		"SSH config invalid for detected machine state",
		func(_ *command.CommandLog) error {
			if machineI.Bootstrap.SSH != nil && isBootstrapped && !machineI.Bootstrap.ForceBootstrap {
				return errors.New("bootstrap SSH is configured but machine is already bootstrapped; set \"force_bootstrap\" to re-bootstrap, or remove bootstrap SSH configuration") //nolint:lll
			}

			if !isBootstrapped && machineI.Bootstrap.SSH == nil && !machineI.Bootstrap.ForceBootstrap {
				return errors.New("bootstrapping requires bootstrap SSH to be configured (or set \"force_bootstrap\" to re-bootstrap with regular SSH)")
			}

			return nil
		},
	)

	return errors.Wrap(err, "SSH config validation failed upon detected machine state")
}

type generationInfo struct {
	Date   string
	Nixos  string
	Kernel string
}

func parseGenerationsOutput(output string) (*machine.Generations, generationInfo, error) {
	lines := strings.Split(output, "\n")

	generations := &machine.Generations{}
	var currentGenInfo generationInfo

	for i, line := range lines {
		genNum, info, isCurrent, ok := parseGenerationLine(i, line)
		if !ok {
			continue
		}

		if isCurrent {
			generations.Current = genNum
			currentGenInfo = info
		}

		generations.Available = append(generations.Available, genNum)
	}

	if len(generations.Available) == 0 {
		return nil, generationInfo{}, errors.New("no generations found - ensure this is a NixOS system with system profiles")
	}

	return generations, currentGenInfo, nil
}

func parseGenerationLine(idx int, line string) (uint, generationInfo, bool, bool) {
	const minFields = 1

	line = strings.TrimSpace(line)
	if line == "" || idx == 0 {
		return 0, generationInfo{}, false, false
	}

	fields := strings.Fields(line)
	if len(fields) < minFields {
		return 0, generationInfo{}, false, false
	}

	parsedNum, err := strconv.ParseUint(fields[0], 10, 0)
	if err != nil {
		return 0, generationInfo{}, false, false
	}

	const currentFieldIdx = 6
	isCurrent := len(fields) > currentFieldIdx && fields[len(fields)-1] == "True"

	var info generationInfo
	if len(fields) >= 2 {
		info.Date = fields[1]
	}

	if len(fields) >= 4 {
		info.Nixos = fields[3]
	}

	if len(fields) >= 5 {
		info.Kernel = fields[4]
	}

	return uint(parsedNum), info, isCurrent, true
}
