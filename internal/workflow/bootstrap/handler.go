package bootstrap

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/pkg/errors"
)

type Handler struct{}

func (Handler) ShouldSkip(fleetLeaf *fleet.FleetLeaf) bool {
	if !fleetLeaf.Installable.Type.IsBootstrappable() {
		return true
	}

	mi := fleetLeaf.Machine.MetaInspect.Load()

	return mi != nil && mi.Bootstrapped && !fleetLeaf.Machine.Bootstrap.ForceBootstrap
}

func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machine := fleetLeaf.Machine

	mi := machine.MetaInspect.Load()
	if mi != nil && mi.RequiresKexec {
		err := executeKexec(exc, machine)
		if err != nil {
			return err
		}
	}

	if !machine.Bootstrap.DisableDisko {
		err := disko(exc, fleetLeaf)
		if err != nil {
			return err
		}
	}

	return errors.Wrap(exc.ExecuteHooks(machine.Bootstrap.PostBootstrapHooks, "post bootstrap hook"), "post bootstrap hook failed")
}
