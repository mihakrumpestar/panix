package workflow

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/acobaugh/osrelease"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var (
	KexecSupportedPlatforms = []string{"x86_64", "aarch64"}
)

func (w *Workflow) executeInspectPhaseMachine(machine *config.Machine) (err error) {
	return w.Phase(machine.Attributes, phases.Inspect, machine,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {
			mms := machine.MetaStatus

			// TCP check

			commandWithArgs := []string{"nc", "-zvw1", machine.Ssh.Hostname, fmt.Sprintf("%d", machine.Ssh.Port)}

			err = exc.Exec(
				"TCP check",
				"unreachable",
				commandWithArgs,
				executioner.SkipIfLocal(),
				executioner.DisableAutoSshCommand(),
				executioner.OnSuccess(func(log *logs.CommandLog) error {
					mms.Reachable.Store(true)
					return nil
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
				executioner.OnFailure(func(log *logs.CommandLog, err error) error {
					return errors.Wrap(err, log.String())
				}),
				executioner.OnSuccess(func(log *logs.CommandLog) error {
					mms.SSHConnectable.Store(true)
					return nil
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
				executioner.OnSuccess(func(log *logs.CommandLog) error {
					architecture := log.String()

					if w.state.Conf.Flags.DryRun {
						architecture = "DRY_RUN"
					}

					architecture = strings.Trim(architecture, "\n")

					if architecture == "" {
						return fmt.Errorf("architecture output was empty")
					}

					if !slices.Contains(KexecSupportedPlatforms, architecture) && !w.state.Conf.Flags.DryRun {
						return fmt.Errorf("platform %s is unsupported, kexec currently only supports %s platforms", strconv.Quote(architecture), KexecSupportedPlatforms)
					}

					mms.Architecture.Store(architecture)
					return nil
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
					func(log *logs.CommandLog, err error) error {
						return errors.Wrap(err, log.String())
					}),
				executioner.OnSuccess(func(log *logs.CommandLog) error {
					output := strings.Trim(log.String(), "\n ")

					parsedOutput, err := strconv.ParseUint(output, 10, 64)
					if err != nil {
						return errors.Wrapf(err, "failed to parse raw output %s to uint", strconv.Quote(output))
					}

					mms.IsRoot.Store(parsedOutput == 0)
					return nil
				}),
			)
			if err != nil {
				return err
			}

			// Run bootstrap detection

			requiresKexec := false
			commandWithArgs = []string{"cat", "/etc/os-release"}

			err = exc.Exec(
				"bootstrap detection",
				"reading /etc/os-release failed",
				commandWithArgs,
				executioner.OnFailure(
					func(log *logs.CommandLog, err error) error {
						return errors.Wrap(err, log.String())
					}),
				executioner.OnSuccess(func(log *logs.CommandLog) error {

					if w.state.Conf.Flags.DryRun {
						mms.Bootstrapped.Store(false)
						return nil
					}

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
						requiresKexec = true
						mms.Bootstrapped.Store(false)
						return nil
					}

					mms.Bootstrapped.Store(true)
					return nil
				}))
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

			// Get current generation
			commandWithArgs = []string{"nixos-rebuild", "list-generations", "--json"}

			err = exc.Exec(
				"get generations",
				"list generations failed",
				commandWithArgs,
				executioner.OnSuccess(func(log *logs.CommandLog) error {
					output := log.Bytes()

					if w.state.Conf.Flags.DryRun {
						output = []byte(`
						[
							{
								"generation": 5,
								"date": "DRY_RUN",
								"nixosVersion": "DRY_RUN",
								"kernelVersion": "DRY_RUN",
								"configurationRevision": "DRY_RUN",
								"specialisations": [],
								"current": true
							}
						]
						`)
					}

					var nixGenerations nixGenerations
					err = json.Unmarshal(output, &nixGenerations)
					if err != nil || len(nixGenerations) == 0 {
						return errors.Wrapf(err, "invalid list-generations output for %s: %s", machine.Name, string(output))
					}

					for _, nixGeneration := range nixGenerations {
						if nixGeneration.Current {
							mms.Generation.Store(nixGeneration.Generation)
							mms.Date.Store(nixGeneration.Date)
							mms.Nixos.Store(nixGeneration.NixosVersion)
							mms.Kernel.Store(nixGeneration.KernelVersion)
							break
						}
					}

					return nil
				}),
			)
			if err != nil {
				return err
			}

			return nil
		})
}

// Unmarshall struct
type nixGenerations []struct {
	Generation            uint32        `json:"generation"`
	Date                  string        `json:"date"`
	NixosVersion          string        `json:"nixosVersion"`
	KernelVersion         string        `json:"kernelVersion"`
	ConfigurationRevision string        `json:"configurationRevision"`
	Specialisations       []interface{} `json:"specialisations"`
	Current               bool          `json:"current"`
}
