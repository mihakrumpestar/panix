package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (w *Workflow) executePreFlakeHookPhaseFlake(flake *config.Flake) error {
	if flake.FlakeHooks.Pre == "" {
		return nil
	}

	return w.Phase(&flake.Attributes, phases.PreFlakeHook, nil,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {
			err := exc.Exec(false, false, nil, nil,
				"sh", "-c", flake.FlakeHooks.Pre)

			return err
		},
	)
}

func (w *Workflow) executePostFlakeHookPhaseFlake(flake *config.Flake) error {
	if flake.FlakeHooks.Post == "" {
		return nil
	}

	return w.Phase(&flake.Attributes, phases.PostFlakeHook, nil,
		func(exc *executioner.Executioner, phaseLog *logs.PhaseLog) error {
			err := exc.Exec(false, false, nil, nil,
				"sh", "-c", flake.FlakeHooks.Post)

			return err
		},
	)
}
