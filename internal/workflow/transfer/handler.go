package transfer

import (
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

type Handler struct{}

func (Handler) ShouldSkip(fleetLeaf *fleet.FleetLeaf) bool {
	// BuildModeRemote + single machine: closure is already on the target
	if fleetLeaf.Configuration.Nix.BuildMode == nix.BuildModeRemote {
		machineCount := 0
		for range fleetLeaf.Configuration.Machines.Pairs() {
			machineCount++
		}

		if machineCount <= 1 {
			return true
		}
	}

	// Local machine: no transfer needed
	return fleetLeaf.Machine.SSH.IsLocal()
}

func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	systemClosure := fleetLeaf.Configuration.MetaBuild.SystemClosure

	return errors.Wrap(phaseops.CopyClosure(exc, fleetLeaf, []string{systemClosure}, true), "transfer failed")
}
