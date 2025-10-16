package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeBootstrapPhaseMachine(machine *config.Machine) error {
	return w.Phase(&machine.Attributes, phases.Bootstrap, machine,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {

			if !w.state.Conf.Flags.Bootstrap.DisableDisko {

				commandWithArgs := []string{machine.Configuration.MetaBuild.DiskoScript}

				err := exc.Exec(commandWithArgs,
					executioner.OnFailure(func(log *logs.CommandLog, err error) error {
						return errors.Wrapf(err, "running diskoScript failed for %s", machine.Name)
					}),
				)
				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}
