package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/pkg/errors"
)

func (w *Workflow) executeActivatePhaseMachine(configuration *config.Configuration, machine *config.Machine) (err error) {
	log := machine.Logs.SafeGet(workflow_definition.PhaseActivate)
	log.TimeAndState.StartTimer()
	defer func() {
		log.TimeAndState.EndTimerWithError(err)
	}()

	if w.state.Conf.Global.Verbose {
		log.AddMessageOnly(fmt.Sprintf("Starting activation of %s", machine.Name))
	}

	buildOutputPath := configuration.Phases.Build.BuildOutputPath

	if w.state.Conf.Global.DryRun {
		return
	}

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, machine, log, w.hook.OnUpdateHook)

	// Build a configuration
	err = exc.Exec(false,
		func(log *config.Log, err error) error {
			return errors.Wrapf(err, "activation failed for %s", machine.Name)
		},
		nil,
		"sudo", buildOutputPath+"/bin/switch-to-configuration", "switch",
	)
	if err != nil {
		return
	}

	if w.state.Conf.Global.Verbose {
		log.AddMessageOnly(fmt.Sprintf("Activated %s successfully", machine.Name))
	}

	return
}
