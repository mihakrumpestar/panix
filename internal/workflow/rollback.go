package workflow

import (
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

var (
	ErrNoGenerations        = errors.New("no generations found")
	ErrGenerationOutOfRange = errors.New("generation number out of range")
)

func (w *Workflow) executeRollbackPhaseMachine(machine *config.Machine) error {
	return w.Phase(machine, phases.Rollback, machine,
		func(exc *executioner.Executioner, phaseLog *phase.PhaseLog) error {
			mi := machine.MetaInspect.Load()
			if mi == nil || mi.Generations == nil || len(mi.Generations.Generations) == 0 {
				return ErrNoGenerations
			}

			genData := mi.Generations

			var targetGenNum uint

			err := exc.ExecFn(
				"validate generation",
				"validating generation number",
				"generation validation failed",
				func(log *command.CommandLog) error {
					var err error

					targetGenNum, err = validateAndGetTargetGen(genData, w.conf.RollbackGeneration)
					if err != nil {
						return err
					}

					log.WriteLineString(fmt.Sprintf("target generation: %d", targetGenNum))

					return nil
				},
			)
			if err != nil {
				return errors.Wrap(err, "generation validation failed")
			}

			return executeRollback(exc, machine, targetGenNum)
		},
	)
}

func validateAndGetTargetGen(genData *config.GenerationsData, generation int) (uint, error) {
	generations := genData.Generations
	currentGen := genData.Current

	switch {
	case generation == 0:
		return currentGen, nil
	case generation < 0:
		resolvedGeneration := int(currentGen) + generation //nolint:gosec // G115: uint -> int should not overflow
		if resolvedGeneration <= 0 {
			return 0, errors.Wrapf(ErrGenerationOutOfRange, "%d", resolvedGeneration)
		}

		return getSpecificGeneration(generations, uint(resolvedGeneration))
	default:
		return getSpecificGeneration(generations, uint(generation))
	}
}

func getSpecificGeneration(generations []*config.GenerationInfo, generation uint) (uint, error) {
	for _, gen := range generations {
		if gen.Number == generation {
			return generation, nil
		}
	}

	return 0, errors.Wrapf(ErrGenerationOutOfRange, "generation %d not found", generation)
}

func executeRollback(exc *executioner.Executioner, machine *config.Machine, targetGenNum uint) error {
	closurePath, err := findGenerationClosure(exc, machine, targetGenNum)
	if err != nil {
		return errors.Wrap(err, "failed to find generation closure")
	}

	err = setSystemProfile(exc, machine, closurePath)
	if err != nil {
		return errors.Wrap(err, "failed to set profile")
	}

	err = activateConfiguration(exc, machine, closurePath, flags.ActivationModeSwitch)
	if err != nil {
		return errors.Wrap(err, "failed to activate")
	}

	return nil
}

func findGenerationClosure(exc *executioner.Executioner, machine *config.Machine, generation uint) (string, error) {
	var closurePath string

	generationLink := fmt.Sprintf("/nix/var/nix/profiles/system-%d-link", generation)

	err := exc.Exec(
		"find generation closure",
		"finding generation closure path",
		"failed to find generation closure",
		append(machine.MaybeSudo(), "readlink", generationLink),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			closurePath = strings.TrimSpace(log.String())

			if closurePath == "" {
				return errors.New("generation closure path is empty")
			}

			return nil
		}),
		executioner.OnDryRun(func() {
			closurePath = "/nix/store/dry-run-closure"
		}),
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute readlink")
	}

	return closurePath, nil
}
