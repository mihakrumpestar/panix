package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

// executeStatusPhase runs status checks in parallel across all machines
// and must complete fully before proceeding to next phase
func (w *Workflow) ExecuteStatusPhase() error {
	return poolChildren(w, w.state.Conf.Root, true, func(flake *config.Flake) error {
		return poolChildren(w, flake, true, func(configuration *config.Configuration) error {
			return poolChildren(w, configuration, true, func(machine *config.Machine) error {
				return w.executeStatusPhaseMachine(machine)
			})
		})
	})
}

func (w *Workflow) executeStatusPhaseMachine(machine *config.Machine) (err error) {
	return w.Phase(machine.Logs.SafeGet(phases.Status),
		fmt.Sprintf("Started stats of %s", machine.Name),
		fmt.Sprintf("Finished stats of %s", machine.Name),
		machine,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {

			defer func() {
				// Disable machine if status fails
				if err != nil {
					machine.Disabled = true
				}
			}()

			sm := machine.MetaStatus

			// TCP check
			err = exc.Exec(true, true,
				func(log *config.CommandLog, err error) error {
					return fmt.Errorf("machine unreachable: %w", err)
				},
				func(log *config.CommandLog) error {
					sm.Reachable = true
					return nil
				},
				"nc", "-zvw1", machine.Ssh.Hostname, fmt.Sprintf("%d", machine.Ssh.Port))
			if err != nil {
				return err
			}

			// SSH connect
			err = exc.Exec(true, true,
				func(log *config.CommandLog, err error) error {
					return errors.Wrapf(err, "ssh test failed: %s", log.String())
				},
				func(log *config.CommandLog) error {
					sm.SSHConnectable = true
					return nil
				}, "echo", "OK")
			if err != nil {
				return err
			}

			// Run bootstrap detection
			err = exc.Exec(false, true,
				nil,
				func(log *config.CommandLog) error {
					sm.Bootstrapped = true
					return nil
				}, "test", "-e", "/run/current-system")
			if err != nil {
				err = nil // just not bootstrapped, not actually an error
				return err
			}

			// Get current generation
			return exc.Exec(false, true,
				nil,
				func(log *config.CommandLog) error {
					output := log.Bytes()

					var nixGenerations nixGenerations
					err = json.Unmarshal(output, &nixGenerations)
					if err != nil || len(nixGenerations) == 0 {
						return errors.Wrapf(err, "invalid list-generations output for %s: %s", machine.Name, string(output)) // strconv.Quote()
					}

					for _, nixGeneration := range nixGenerations {
						if nixGeneration.Current {
							sm.Generation = fmt.Sprint(nixGeneration.Generation)
							sm.Date = nixGeneration.Date
							sm.Nixos = nixGeneration.NixosVersion
							sm.Kernel = nixGeneration.KernelVersion
							break
						}
					}

					return nil
				}, "nixos-rebuild", "list-generations", "--json")
		})
}

// Unmarshall struct
type nixGenerations []struct {
	Generation            int           `json:"generation"`
	Date                  string        `json:"date"`
	NixosVersion          string        `json:"nixosVersion"`
	KernelVersion         string        `json:"kernelVersion"`
	ConfigurationRevision string        `json:"configurationRevision"`
	Specialisations       []interface{} `json:"specialisations"`
	Current               bool          `json:"current"`
}
