package build

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

type Handler struct{}

func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	flake := fleetLeaf.Flake
	configurationI := fleetLeaf.Configuration

	if configurationI.MetaBuild == nil {
		configurationI.MetaBuild = &configuration.MetaBuild{}
	}

	flakeOutput := configuration.ResolveFlakeInstallable(configurationI.FlakeOutput, configurationI.BuildPath, configurationI.Name)
	installables := []string{fmt.Sprintf("%s#%s", flake.URL, flakeOutput)}

	storePath, err := phaseops.BuildInstallable(exc, fleetLeaf, installables, "system closure")
	if err != nil {
		return errors.Wrap(err, "system closure build failed")
	}

	configurationI.MetaBuild.SystemClosure = storePath

	return nil
}
