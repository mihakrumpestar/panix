package workflow

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/pkg/errors"
)

// executeStatusPhase runs status checks in parallel across all machines
// and must complete fully before proceeding to next phase
func (w *Workflow) ExecuteStatusPhase() error {
	if w.state.Conf.Global.Verbose {
		fmt.Println("Executing status phase across all flake configurations")
	}

	err := w.forEachFlakeConfiguration(func(groupPool pond.TaskGroup, flakeName string, configurationName string, flake *config.Flake, configuration *config.Configuration) error {
		if w.state.Conf.Global.Verbose {
			fmt.Println("Executing status phase across all machines in " + flakeName + " " + configurationName)
		}

		err := w.forEachConfigurationMachine(nil, flakeName, configurationName, configuration, func(machineName url.URL, machine *config.Machine) error {
			if w.state.Conf.Global.Verbose {
				fmt.Println("Executing status phase on machine " + machineName.String())
			}

			machineStatusTimeAndState := machine.Logs[workflow_definition.PhaseStatus].TimeAndState

			machineStatusTimeAndState.StartTimer()
			err := w.executeStatusPhaseMachine(machineName, machine)
			machineStatusTimeAndState.EndTimerWithError(err)

			if w.state.Conf.Global.Verbose {
				fmt.Println("Executing finished for status phase on machine " + machineName.String())
			}

			return err
		})

		if w.state.Conf.Global.Verbose {
			fmt.Println("Executing finished for status phase across all machines in " + flakeName + " " + configurationName)
		}

		return err
	})

	if w.state.Conf.Global.Verbose {
		fmt.Println("Executing finished for status phase with err %w", err)
	}

	w.hook.OnUpdateHook()

	return err
}

func (w *Workflow) executeStatusPhaseMachine(machineName url.URL, machine *config.Machine) error {
	log := machine.Logs[workflow_definition.PhaseStatus]

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, &machineName, machine.Ssh, log, w.hook.OnUpdateHook)

	sm := machine.Phases.Status

	// TCP check
	err := exc.PingStream(
		func(log *config.Log, err error) error {
			return fmt.Errorf("machine unreachable: %w", err)
		},
		func(log *config.Log) error {
			sm.Reachable = true
			return nil
		})
	if err != nil {
		return err
	}

	// SSH connect
	err = exc.Exec(
		func(log *config.Log, err error) error {
			return errors.Wrapf(err, "ssh test failed: %s", log.LastCommand().StdInOutErr.String())
		},
		func(log *config.Log) error {
			sm.SSHConnectable = true
			return nil
		}, "sh", "-c", "exit 0")
	if err != nil {
		return err
	}

	// Run bootstrap detection
	err = exc.Exec(
		nil,
		func(log *config.Log) error {
			sm.Bootstrapped = true
			return nil
		}, "sh", "-c", "test -e /run/current-system")
	if err != nil {
		return nil // just not bootstrapped, not really an error
	}

	// Get current generation
	err = exc.Exec(
		nil,
		func(log *config.Log) error {
			sm.CurrentGeneration = strings.TrimSpace(log.LastCommand().StdInOutErr.String())
			return nil
		}, "sh", "-c", "sleep 5 && nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
	if err != nil {
		return err
	}

	// Get last deploy time
	err = exc.Exec(
		nil,
		func(log *config.Log) error {
			sm.LastDeployTime = strings.TrimSpace(log.LastCommand().StdInOutErr.String())
			return nil
		}, "sh", "-c", "stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
	if err != nil {
		return err
	}

	return nil
}
