package secrets

import (
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

	for _, secret := range secrets {
		err := phaseops.TransferSecret(exc, machine, secret, "secrets", true)
		if err != nil {
			// Command-only sources have no local path: name them by command.
			secretName := secret.LocalPath
			if secretName == "" {
				secretName = secret.Command
			}

			return errors.Wrapf(err, "failed to transfer secret %s", secretName)
		}
	}

	return nil
}
