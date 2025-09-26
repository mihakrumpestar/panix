package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/pkg/errors"
)

// This function is called by executeMachineTransfer for individual configurations -> machine transfers
func (w *Workflow) executeTransferPhaseMachine(configuration *config.Configuration, machine *config.Machine) (err error) {
	log := machine.Logs.SafeGet(workflow_definition.PhaseTransfer)
	log.TimeAndState.StartTimer()
	defer func() {
		log.TimeAndState.EndTimerWithError(err)
	}()

	buildOutputPath := configuration.Phases.Build.BuildOutputPath

	if buildOutputPath == "" {
		err = fmt.Errorf("machine %s has no build output path", machine.Name)
		return
	}

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, nil, log, w.hook.OnUpdateHook)

	err = exc.Exec(true, true,
		func(l *config.Log, err error) error {
			return errors.Wrap(err, "nix copy failed")
		},
		nil,
		"nix", "copy", "--to", machine.Name, buildOutputPath)
	if err != nil {
		return
	}

	if w.state.Conf.Global.Verbose {
		log.AddMessageOnly(fmt.Sprintf("Transferred %s to %s\n", buildOutputPath, machine.Name))
	}

	return
}
