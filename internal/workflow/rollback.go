package workflow

import (
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
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

func (w *Workflow) executeRollbackPhaseMachine(fleetLeaf *fleet.FleetLeaf) error {
	return w.Phase(phases.Rollback, fleetLeaf,
		func(exc *executioner.Executioner, phaseLog *phase.PhaseLog) error {
			machine := fleetLeaf.Machine

			mi := machine.MetaInspect.Load()
			if mi == nil || mi.Generations == nil || len(mi.Generations.Available) == 0 {
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

					targetGenNum, err = validateAndGetTargetGen(genData, w.conf.Flags.Generation)
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

func validateAndGetTargetGen(generations *machine.Generations, rollbackGeneration int) (uint, error) {
	current := generations.Current
	availableGenerations := generations.Available

	switch {
	case rollbackGeneration == 0:
		return current, nil
	case rollbackGeneration < 0:
		resolvedGeneration := int(current) + rollbackGeneration //nolint:gosec // G115: uint -> int should not overflow
		if resolvedGeneration <= 0 {
			return 0, errors.Wrapf(ErrGenerationOutOfRange, "%d", resolvedGeneration)
		}

		return getSpecificGeneration(availableGenerations, uint(resolvedGeneration))
	default:
		return getSpecificGeneration(availableGenerations, uint(rollbackGeneration))
	}
}

func getSpecificGeneration(availableGenerations []uint, specificGeneration uint) (uint, error) {
	for _, availableGeneration := range availableGenerations {
		if availableGeneration == specificGeneration {
			return specificGeneration, nil
		}
	}

	return 0, errors.Wrapf(ErrGenerationOutOfRange, "generation %d not found", specificGeneration)
}

func executeRollback(exc *executioner.Executioner, machine *machine.Machine, targetGenNum uint) error {
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

func findGenerationClosure(exc *executioner.Executioner, machine *machine.Machine, generation uint) (string, error) {
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
