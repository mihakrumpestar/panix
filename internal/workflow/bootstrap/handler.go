package bootstrap

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

type Handler struct {
	OutLinks phaseops.OutLinks
}

func (Handler) ShouldSkip(fleetLeaf *fleet.FleetLeaf) bool {
	if !fleetLeaf.Installable.Preset.IsBootstrappable {
		return true
	}

	mi := fleetLeaf.Machine.MetaInspect.Load()

	return mi != nil && mi.Bootstrapped && !fleetLeaf.Machine.Bootstrap.ForceBootstrap
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machine := fleetLeaf.Machine

	mi := machine.MetaInspect.Load()
	if mi != nil && mi.RequiresKexec {
		err := executeKexec(exc, machine)
		if err != nil {
			return err
		}
	}

	if !machine.Bootstrap.DisableDisko {
		err := disko(exc, fleetLeaf, h.OutLinks.DiskoPath(fleetLeaf.Installable))
		if err != nil {
			return err
		}
	}

	return errors.Wrap(exc.ExecuteHooks(machine.Bootstrap.PostBootstrapHooks, "post bootstrap hook"), "post bootstrap hook failed")
}
