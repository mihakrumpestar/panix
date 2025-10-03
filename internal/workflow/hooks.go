package workflow

import (
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (w *Workflow) executePreFlakeHokPhaseFlake(flake *config.Flake) error {
	return w.Phase(flake.Logs.SafeGet(phases.PreFlakeHook),
		fmt.Sprintf("Started preFlakeHook of %s", flake.Name),
		fmt.Sprintf("Finished preFlakeHook of %s", flake.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {
			commandWithArgs := []string{"sh", "-c", flake.BuildHooks.Pre}

			if flake.BuildHooks.Pre == "" {
				phaseLog.AddMessageOnly("(skipped) ", strings.Join(commandWithArgs, " "))
			}

			err := exc.Exec(false, true, nil, nil, commandWithArgs...)

			return err
		},
	)
}

func (w *Workflow) executePostFlakeHokPhaseFlake(flake *config.Flake) error {
	return w.Phase(flake.Logs.SafeGet(phases.PostFlakeHook),
		fmt.Sprintf("Started postFlakeHook of %s", flake.Name),
		fmt.Sprintf("Finished postFlakeHook of %s", flake.Name),
		nil,
		func(exc *executioner.Executioner, phaseLog *config.PhaseLog) error {
			commandWithArgs := []string{"sh", "-c", flake.BuildHooks.Post}

			if flake.BuildHooks.Pre == "" {
				phaseLog.AddMessageOnly("(skipped) ", strings.Join(commandWithArgs, " "))
			}

			err := exc.Exec(false, true, nil, nil, commandWithArgs...)

			return err
		},
	)
}
