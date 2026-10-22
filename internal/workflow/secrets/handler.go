package secrets

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

type Handler struct{}

func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machine := fleetLeaf.Machine

	secrets := machine.Secrets
	if len(secrets) == 0 {
		return nil
	}

	// OS secrets follow the NixOS bootstrapping root (if any) so they land
	// on the future root while bootstrapping, and on the live root otherwise.
	bootstrapsNixOS := fleetLeaf.Installable.Preset.BootstrapsNixOS()

	for i, secret := range secrets {
		err := phaseops.TransferSecret(exc, machine, secret, "secrets", bootstrapsNixOS)
		if err != nil {
			return errors.Wrapf(err, "failed to transfer secret %s", secretTransferLabel(i, secret))
		}
	}

	return nil
}

// secretTransferLabel names a failed transfer without ever rendering the
// command text, which may itself contain secret material. Command-only sources
// get a stable index-based label; file sources keep their path.
func secretTransferLabel(index int, secret attributes.TransferSource) string {
	if secret.LocalPath != "" {
		return secret.LocalPath
	}

	return fmt.Sprintf("command source #%d", index+1)
}
