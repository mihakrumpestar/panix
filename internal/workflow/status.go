package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
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
	log := machine.Logs.SafeGet(workflow_definition.PhaseStatus)
	log.TimeAndState.StartTimer()
	defer func() {
		// Disable machine if status fails
		if err != nil {
			machine.Disabled = true
		}

		log.TimeAndState.EndTimerWithError(err)
	}()

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, machine, log, w.hook.OnUpdateHook)

	sm := machine.Phases.Status

	// TCP check
	err = exc.Exec(true,
		func(log *config.Log, err error) error {
			return fmt.Errorf("machine unreachable: %w", err)
		},
		func(log *config.Log) error {
			sm.Reachable = true
			return nil
		},
		"nc", "-zvw1", machine.Ssh.Hostname, string(machine.Ssh.Port))
	if err != nil {
		return
	}

	// SSH connect
	err = exc.Exec(true,
		func(log *config.Log, err error) error {
			return errors.Wrapf(err, "ssh test failed: %s", log.LastCommand().String())
		},
		func(log *config.Log) error {
			sm.SSHConnectable = true
			return nil
		}, "echo", "OK")
	if err != nil {
		return
	}

	// Run bootstrap detection
	err = exc.Exec(false,
		nil,
		func(log *config.Log) error {
			sm.Bootstrapped = true
			return nil
		}, "test", "-e", "/run/current-system")
	if err != nil {
		err = nil // just not bootstrapped, not actually an error
		return
	}

	// Get current generation
	err = exc.Exec(false,
		nil,
		func(log *config.Log) error {
			output := log.LastCommand().Bytes()

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
	if err != nil {
		return
	}

	return
}

// Helpers

type nixGenerations []struct {
	Generation            int           `json:"generation"`
	Date                  string        `json:"date"`
	NixosVersion          string        `json:"nixosVersion"`
	KernelVersion         string        `json:"kernelVersion"`
	ConfigurationRevision string        `json:"configurationRevision"`
	Specialisations       []interface{} `json:"specialisations"`
	Current               bool          `json:"current"`
}
