package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeBootstrapPhaseMachine(configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Logs.SafeGet(phases.Bootstrap),
		fmt.Sprintf("Started bootstrap of %s", machine.Name),
		fmt.Sprintf("Finished bootstrap of %s", machine.Name),
		machine,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {

			if !w.state.Conf.Global.Bootstrap.DisableDisko {
				err := exc.Exec(false, true,
					func(log *config.CommandLog, err error) error {
						return errors.Wrapf(err, "running diskoScript failed for %s", machine.Name)
					},
					nil,
					configuration.MetaBuild.DiskoScript,
				)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}
