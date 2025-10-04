package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (w *Workflow) executePreFlakeHookPhaseFlake(flake *config.Flake) error {
	if flake.FlakeHooks.Pre == "" {
		return nil
	}

	return w.Phase(flake.Logs.SafeGet(phases.PreFlakeHook),
		fmt.Sprintf("Started preFlakeHook of %s", flake.Name),
		fmt.Sprintf("Finished preFlakeHook of %s", flake.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {
			err := exc.Exec(false, true, nil, nil,
				"sh", "-c", flake.FlakeHooks.Pre)

			return err
		},
	)
}

func (w *Workflow) executePostFlakeHookPhaseFlake(flake *config.Flake) error {
	if flake.FlakeHooks.Post == "" {
		return nil
	}

	return w.Phase(flake.Logs.SafeGet(phases.PostFlakeHook),
		fmt.Sprintf("Started postFlakeHook of %s", flake.Name),
		fmt.Sprintf("Finished postFlakeHook of %s", flake.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {
			err := exc.Exec(false, true, nil, nil,
				"sh", "-c", flake.FlakeHooks.Post)

			return err
		},
	)
}
