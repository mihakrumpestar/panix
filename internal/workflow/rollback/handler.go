package rollback

import (
	"fmt"
	"slices"

	"github.com/ccoveille/go-safecast/v2"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/pkg/errors"
)

type Handler struct {
	TargetGeneration int
	NixFlavor        nixver.Flavor
}

// ShouldSkip returns true for installable types that don't have versioned
// profiles (i.e. packages, which are installed via nix profile add
// and have no generation concept).
func (Handler) ShouldSkip(fleetLeaf *fleet.FleetLeaf) bool {
	return fleetLeaf.Installable.Preset.ProfilePath == ""
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machineI := fleetLeaf.Machine

	mi := machineI.MetaInspect.Load()
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

	return executeRollback(exc, fleetLeaf, targetGenNum, h.NixFlavor)
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

func executeRollback(
	exc *executioner.Executioner,
	fleetLeaf *fleet.FleetLeaf,
	targetGenNum uint,
	nixFlavor nixver.Flavor,
) error {
	preset := fleetLeaf.Installable.Preset

	closurePath, err := phaseops.FindGenerationClosure(exc, fleetLeaf.Machine, preset.ProfilePath, targetGenNum)
	if err != nil {
		return err //nolint:wrapcheck // error is pre-annotated with statusIfFailed
	}

	return errors.Wrap(
		phaseops.Activate(
			exc,
			fleetLeaf.Machine,
			preset,
			closurePath,
			phaseops.RollbackActivationMode(preset),
			fleetLeaf.Installable.User,
			&fleetLeaf.Installable.Nix,
			nixFlavor,
		),
		"failed to activate rollback generation",
	)
}
