package workflow

import (
	"fmt"
	"net/url"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/pkg/errors"
)

func (w *Workflow) executeActivatePhaseMachine(configuration *config.Configuration, machineName url.URL, machine *config.Machine) (err error) {
	log := machine.Logs.SafeGet(workflow_definition.PhaseActivate)
	log.TimeAndState.StartTimer()
	defer log.TimeAndState.EndTimerWithError(err)

	buildOutputPath := configuration.Phases.Build.BuildOutputPath

	if w.state.Conf.Global.DryRun {
		return
	}

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, &machineName, machine.Ssh, log, w.hook.OnUpdateHook)

	// Build a configuration
	err = exc.Exec(false,
		func(log *config.Log, err error) error {
			return errors.Wrapf(err, "activation failed for %s", machineName.String())
		},
		nil,
		"sudo", fmt.Sprintf("%s/bin/switch-to-configuration", buildOutputPath), "switch",
	)
	if err != nil {
		return
	}

	if w.state.Conf.Global.Verbose {
		fmt.Printf("Activated %s successfully", machineName.String())
	}

	return
}
