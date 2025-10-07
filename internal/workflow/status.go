package workflow

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var (
	KexecSupportedPlatforms = []string{"x86_64", "aarch64"}
)

func (w *Workflow) executeStatusPhaseMachine(machine *config.Machine) (err error) {
	return w.Phase(&machine.Attributes, phases.Status, machine,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {
			mms := machine.MetaStatus

			// TCP check
			err = exc.Exec(true, true,
				func(log *logs.CommandLog, err error) error {
					return fmt.Errorf("machine unreachable: %w", err)
				},
				func(log *logs.CommandLog) error {
					mms.Reachable.Store(true)
					return nil
				},
				"nc", "-zvw1", machine.Ssh.Hostname, fmt.Sprintf("%d", machine.Ssh.Port))
			if err != nil {
				return err
			}

			// SSH connect
			err = exc.Exec(true, false,
				func(log *logs.CommandLog, err error) error {
					return errors.Wrapf(err, "ssh test failed: %s", log.String())
				},
				func(log *logs.CommandLog) error {
					mms.SSHConnectable.Store(true)
					return nil
				}, "echo", "OK")
			if err != nil {
				return err
			}

			// Architecture
			err = exc.Exec(false, false,
				nil,
				func(log *logs.CommandLog) error {
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
				}, "uname", "-m")
			if err != nil {
				return err
			}

			// Run bootstrap detection
			err = exc.Exec(false, false, nil,
				func(log *logs.CommandLog) error {
					bootstrapped := true

					if w.state.Conf.Flags.DryRun {
						bootstrapped = false
					}

					mms.Bootstrapped.Store(bootstrapped)
					return nil
				}, "test", "-e", "/run/current-system")
			if err != nil {
				err = nil // just not bootstrapped, not actually an error

				// Bootstrap part

				/*

					"x86_64 | aarch64)  kexecUrl="https://github.com/nix-community/nixos-images/releases/download/nixos-25.05/nixos-kexec-installer-noninteractive-${isArch}-linux.tar.gz"
					"TMPDIR=/root/kexec setsid --wait ${maybeSudo} /root/kexec/kexec/run"
					 ssh into kexec

				*/

				if !mms.Bootstrapped.Load() && machine.Attributes.HardwareConfigPath != "" {
					commandWithArgs := []string{"sh", "-c", *machine.Attributes.SudoProgram, "nixos-generate-config", "--show-hardware-config", "--no-filesystems", ">", machine.Attributes.HardwareConfigPath}

					err := exc.Exec(false, false,
						func(log *logs.CommandLog, err error) error {
							return errors.Wrapf(err, "nixos-generate-config failed for %s", machine.Attributes.Name)
						},
						nil,
						commandWithArgs...,
					)
					if err != nil {
						return nil
					}
				}

				return err
			}

			// Get current generation
			err = exc.Exec(false, false, nil,
				func(log *logs.CommandLog) error {
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
						return errors.Wrapf(err, "invalid list-generations output for %s: %s", machine.Attributes.Name, string(output)) // strconv.Quote()
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
				}, "nixos-rebuild", "list-generations", "--json")
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
