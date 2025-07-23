package workflow

import (
	"fmt"
	"net/url"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/pkg/errors"
)

// This function is called by executeMachineTransfer for individual configurations -> machine transfers
func (w *Workflow) executeTransferPhaseMachine(configuration *config.Configuration, machineName url.URL, machine *config.Machine) (err error) {
	log := machine.Logs.SafeGet(workflow_definition.PhaseTransfer)
	log.TimeAndState.StartTimer()
	defer func() {
		log.TimeAndState.EndTimerWithError(err)
	}()

	buildOutputPath := configuration.Phases.Build.BuildOutputPath

	if buildOutputPath == "" {
		err = fmt.Errorf("machine %s has no build output path", machineName.String())
		return
	}

	exc := executioner.NewExecutioner(w.ctx, &w.state.Conf.Global, nil, machine.Ssh, log, w.hook.OnUpdateHook)

	err = exc.Exec(true,
		func(l *config.Log, err error) error {
			return errors.Wrap(err, "nix copy failed")
		},
		nil,
		"nix", "copy", "--to", machineName.String(), buildOutputPath)
	if err != nil {
		return
	}

	if w.state.Conf.Global.Verbose {
		log.AddMessageOnly(fmt.Sprintf("Transferred %s to %s\n", buildOutputPath, machineName.String()))
	}

	return
}
