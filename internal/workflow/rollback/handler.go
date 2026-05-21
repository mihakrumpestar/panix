package rollback

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ccoveille/go-safecast/v2"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

type Handler struct {
	TargetGeneration int
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
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

			targetGenNum, err = validateAndGetTargetGen(genData, h.TargetGeneration)
			if err != nil {
				return err
			}

			log.Output.Write(fmt.Appendf(nil, "target generation: %d", targetGenNum))

			return nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "generation validation failed")
	}

	return executeRollback(exc, machine, targetGenNum)
}

var (
	ErrNoGenerations        = errors.New("no generations found")
	ErrGenerationOutOfRange = errors.New("generation number out of range")
)

func validateAndGetTargetGen(generations *machine.Generations, rollbackGeneration int) (uint, error) {
	current := generations.Current
	availableGenerations := generations.Available

	switch {
	case rollbackGeneration == 0:
		return current, nil
	case rollbackGeneration < 0:
		currentInt, err := safecast.Convert[int](current)
		if err != nil {
			return 0, err
		}

		resolvedGeneration := currentInt + rollbackGeneration
		if resolvedGeneration <= 0 {
			return 0, errors.Wrapf(ErrGenerationOutOfRange, "%d", resolvedGeneration)
		}

		return getSpecificGeneration(availableGenerations, uint(resolvedGeneration))
	default:
		return getSpecificGeneration(availableGenerations, uint(rollbackGeneration))
	}
}

func getSpecificGeneration(availableGenerations []uint, specificGeneration uint) (uint, error) {
	if slices.Contains(availableGenerations, specificGeneration) {
		return specificGeneration, nil
	}

	return 0, errors.Wrapf(ErrGenerationOutOfRange, "generation %d not found", specificGeneration)
}

func executeRollback(exc *executioner.Executioner, machine *machine.Machine, targetGenNum uint) error {
	closurePath, err := findGenerationClosure(exc, machine, targetGenNum)
	if err != nil {
		return errors.Wrap(err, "failed to find generation closure")
	}

	err = phaseops.SetSystemProfile(exc, machine, closurePath)
	if err != nil {
		return errors.Wrap(err, "failed to set profile")
	}

	err = phaseops.ActivateConfiguration(exc, machine, closurePath, attributes.ActivationModeSwitch)
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
			closurePath = strings.TrimSpace(log.Output.String())

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
