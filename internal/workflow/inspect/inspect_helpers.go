package inspect

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/osrelease"
	"github.com/mihakrumpestar/panix/pkg/stringbyte"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var (
	ErrArchitectureOutputEmpty = errors.New("architecture output was empty")
	ErrPlatformUnsupported     = errors.New("platform unsupported, kexec supports limited platforms")
)

func checkSSHReachability(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.ExecFn(
		"SSH reachability check",
		"checking SSH reachability",
		"SSH unreachable",
		func(_ *command.CommandLog) error {
			if machineI.Bootstrap.SSH.IsInitialized() {
				bootstrapSSHReachable := machineI.Bootstrap.SSH.ReachabilityCheck(exc.SSHReachabilityTimeout())
				if !bootstrapSSHReachable {
					return errors.New("bootstrap SSH is configured but unreachable")
				}

				err := machineI.Bootstrap.SSH.KnownHostsFile.Create()
				if err != nil {
					return errors.Wrap(err, "failed to create temp known_hosts file")
				}

				machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeBootstrap })
			} else {
				regularSSHReachable := machineI.SSH.ReachabilityCheck(exc.SSHReachabilityTimeout())

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
		executioner.OnFailure(func(log *command.CommandLog, err error) error {
			return errors.Wrap(err, log.Output.String())
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
			architecture := strings.Trim(log.Output.String(), "\n")
			if architecture == "" {
				return ErrArchitectureOutputEmpty
			}

			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Architecture = stringbyte.StringByte(architecture)
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Architecture = stringbyte.StringByte("DRY_RUN")
			})
		}),
	)
	if err != nil {
		return errors.Wrap(err, "architecture detection failed")
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
			return errors.Wrap(err, log.Output.String())
		}),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			return classifyBootstrapStatus(log.Output.String(), machineI)
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Bootstrapped = false
				mi.RequiresKexec = true
				mi.OSVersion = stringbyte.StringByte("DRY_RUN")
			})
		}),
	)
	if err != nil {
		return errors.Wrap(err, "bootstrap detection failed")
	}

	return nil
}

func checkNixAvailable(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.Exec(
		"nix check",
		"checking nix availability",
		"nix not found on remote machine",
		[]string{"nix", "--version"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.NixAvailable = true
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.NixAvailable = true
			})
		}),
	)

	return errors.Wrap(err, "nix availability check failed")
}

// detectSystemInfo skips Date: readGenerations populates it from the active
// generation.
func detectSystemInfo(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := detectOSVersion(exc, machineI)
	if err != nil {
		return err
	}

	return detectKernel(exc, machineI)
}

func detectOSVersion(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.Exec(
		"system info",
		"detecting system info",
		"system info detection failed",
		[]string{"cat", "/etc/os-release"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			osr, err := osrelease.ReadString(log.Output.String())
			if err != nil {
				return errors.Wrap(err, "error parsing /etc/os-release")
			}

			machineI.MetaInspect.Update(func(metaInspect *machine.MetaInspect) {
				version, ok := osr["VERSION"]
				if ok {
					metaInspect.OSVersion = stringbyte.StringByte(version)
				} else {
					pretty, ok2 := osr["PRETTY_NAME"]
					if ok2 {
						metaInspect.OSVersion = stringbyte.StringByte(pretty)
					}
				}
			})

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.OSVersion = stringbyte.StringByte("DRY_RUN")
			})
		}),
	)

	return errors.Wrap(err, "system info detection failed")
}

func detectKernel(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.Exec(
		"kernel version",
		"detecting kernel version",
		"kernel detection failed",
		[]string{"uname", "-r"},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			kernel := strings.TrimSpace(log.Output.String())
			if kernel != "" {
				machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
					mi.Kernel = stringbyte.StringByte(kernel)
				})
			}

			return nil
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.Kernel = stringbyte.StringByte("DRY_RUN")
			})
		}),
	)

	return errors.Wrap(err, "kernel detection failed")
}

func classifyBootstrapStatus(output string, machineI *machine.Machine) error {
	osr, err := osrelease.ReadString(output)
	if err != nil {
		return errors.Wrap(err, "error parsing /etc/os-release")
	}

	if osr["ID"] == "nixos" && osr["VARIANT_ID"] == "installer" {
		machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
			mi.Bootstrapped = false
			mi.OSVersion = stringbyte.StringByte(osr["VERSION"])
		})

		return nil
	}

	if osr["ID"] != "nixos" {
		machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
			mi.RequiresKexec = true
			mi.Bootstrapped = false
			mi.OSVersion = stringbyte.StringByte(osr["VERSION"])
		})

		err = machineI.Bootstrap.Kexec.Image.IfDefaultImageIsArchSupported(machineI.MetaInspect.Load().Architecture.String())
		if err != nil {
			return errors.Wrap(err, "kexec arch validation failed")
		}

		return nil
	}

	if machineI.Bootstrap.ForceBootstrap {
		machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
			mi.Bootstrapped = false
			mi.RequiresKexec = machineI.Bootstrap.ForceBootstrapKexec
			mi.OSVersion = stringbyte.StringByte(osr["VERSION"])
		})

		if machineI.Bootstrap.ForceBootstrapKexec {
			err = machineI.Bootstrap.Kexec.Image.IfDefaultImageIsArchSupported(machineI.MetaInspect.Load().Architecture.String())
			if err != nil {
				return errors.Wrap(err, "kexec arch validation failed")
			}
		}

		return nil
	}

	machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
		mi.Bootstrapped = true
		mi.OSVersion = stringbyte.StringByte(osr["VERSION"])
	})

	return nil
}

// handleUnbootstrapped generates the hardware config during Inspect, via
// phaseops so Bootstrap runs the same step after kexec.
func handleUnbootstrapped(exc *executioner.Executioner, machineI *machine.Machine) error {
	if machineI.HardwareConfigPath == "" {
		return nil
	}

	// nixos-generate-config exists only on NixOS: skip before kexec boots
	// the installer; Bootstrap generates it afterwards.
	mi := machineI.MetaInspect.Load()
	if mi != nil && mi.RequiresKexec {
		log.Info().
			Str("xpath", machineI.Xpath.String()).
			Str("hardware_config_path", machineI.HardwareConfigPath).
			Msg("skipping hardware config generation before kexec (nixos-generate-config is unavailable on the current OS)")

		return nil
	}

	return phaseops.GenerateHardwareConfig(exc, machineI) //nolint:wrapcheck // error is pre-annotated with its own context
}

// readGenerations works for any profile path (system, home-manager,
// system-manager, ...). NIX_PAGER=cat: panix runs on a PTY and nix's pager
// would block forever (#14).
func readGenerations(
	exc *executioner.Executioner,
	machineI *machine.Machine,
	preset installable.Preset,
	profilePath string,
	targetUser string,
) error {
	// The env prefix stays inline (not WithEnv) so it sits inside the
	// privilege wrap regardless of which user runs nix-env.
	args := phaseops.WrapAsTargetUser(machineI, preset, targetUser,
		[]string{"env", "NIX_PAGER=cat", "nix-env", "--profile", profilePath, "--list-generations"})

	err := exc.Exec(
		"list generations",
		"listing generations",
		"failed to list generations",
		args,
		executioner.OnSuccess(func(log *command.CommandLog) error {
			genData, date := parseNixEnvGenerations(log.Output.String())

			machineI.MetaInspect.Update(func(metaInspect *machine.MetaInspect) {
				metaInspect.Generations = genData
				if date != "" {
					metaInspect.Date = stringbyte.StringByte(date)
				}
			})

			return nil
		}),
		executioner.OnFailure(func(_ *command.CommandLog, err error) error {
			// Exit 1 (no profile yet) and 127 (nix-env not in PATH, e.g.
			// kexec VM) are expected; anything else propagates.
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code := exitErr.ExitCode()
				if code == 1 || code == 127 {
					return nil
				}
			}

			return err
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(metaInspect *machine.MetaInspect) {
				metaInspect.Generations = &machine.Generations{
					Current:   1,
					Available: []uint{1},
				}
			})
		}),
	)

	return errors.Wrap(err, "failed to list generations")
}

// parseNixEnvGenerations expects one generation per line, current marked:
//
//	1   2026-08-01 15:00:44
//	2   2026-08-01 15:10:22   (current)
func parseNixEnvGenerations(output string) (*machine.Generations, string) {
	lines := strings.Split(output, "\n")

	generations := &machine.Generations{}

	var currentDate string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		genNum, err := strconv.ParseUint(fields[0], 10, 0)
		if err != nil {
			continue
		}

		generations.Available = append(generations.Available, uint(genNum))

		isCurrent := strings.Contains(line, "(current)")
		if isCurrent {
			generations.Current = uint(genNum)

			if len(fields) >= 3 { //nolint:mnd
				currentDate = fields[1] + " " + fields[2]
			}
		}
	}

	if len(generations.Available) == 0 {
		return nil, "" // no generations yet, not an error
	}

	if generations.Current == 0 && len(generations.Available) > 0 {
		generations.Current = generations.Available[len(generations.Available)-1]
	}

	return generations, currentDate
}

func validateSSHMachineState(exc *executioner.Executioner, machineI *machine.Machine) error {
	mi := machineI.MetaInspect.Load()
	isBootstrapped := mi != nil && mi.Bootstrapped

	err := exc.ExecFn(
		"validate SSH config upon detected machine state",
		"validating SSH config",
		"SSH config invalid for detected machine state",
		func(_ *command.CommandLog) error {
			if machineI.Bootstrap.SSH.IsInitialized() && isBootstrapped && !machineI.Bootstrap.ForceBootstrap {
				return errors.New("bootstrap SSH is configured but machine is already bootstrapped, set \"force_bootstrap\" to re-bootstrap, or remove bootstrap SSH configuration") //nolint:lll
			}

			if !isBootstrapped && !machineI.Bootstrap.SSH.IsInitialized() && !machineI.Bootstrap.ForceBootstrap {
				return errors.New("bootstrapping requires bootstrap SSH to be configured (or set \"force_bootstrap\" to re-bootstrap with regular SSH)")
			}

			return nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "SSH config validation failed upon detected machine state")
	}

	return nil
}

func validateSecretsPaths(exc *executioner.Executioner, machineI *machine.Machine) error {
	mi := machineI.MetaInspect.Load()
	isBootstrapped := mi != nil && mi.Bootstrapped

	if isBootstrapped {
		return nil
	}

	err := exc.ExecFn(
		"bootstrap secrets paths",
		"validating bootstrap secrets paths",
		"missing bootstrap secrets paths",
		func(_ *command.CommandLog) error {
			return errors.Wrap(machineI.ValidateBootstrapSecretsPaths(), "bootstrap secrets path validation failed")
		},
	)
	if err != nil {
		return errors.Wrap(err, "bootstrap secrets paths validation failed")
	}

	return nil
}
