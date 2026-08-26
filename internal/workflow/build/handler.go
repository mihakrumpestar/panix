package build

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

type Handler struct {
	OutLinks phaseops.OutLinks
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	flake := fleetLeaf.Flake

	if fleetLeaf.Installable.MetaBuild == nil {
		fleetLeaf.Installable.MetaBuild = &installable.MetaBuild{}
	}

	flakeOutput := installable.ResolveFlakeInstallable(fleetLeaf.Installable.Type, fleetLeaf.Installable.Name, fleetLeaf.Installable.Preset)
	installables := []string{fmt.Sprintf("%s#%s", flake.URL, flakeOutput)}

	storePath, err := phaseops.BuildInstallable(exc, fleetLeaf, installables, "system closure", h.OutLinks.ClosurePath(fleetLeaf.Installable))
	if err != nil {
		return errors.Wrap(err, "system closure build failed")
	}

	fleetLeaf.Installable.MetaBuild.Closure = storePath

	return nil
}
