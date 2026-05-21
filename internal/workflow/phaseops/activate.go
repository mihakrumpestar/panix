package phaseops

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/pkg/errors"
)

func SetSystemProfile(exc *executioner.Executioner, machine *machine.Machine, closurePath string) error {
	err := exc.Exec(
		"set system profile",
		"setting system profile",
		"failed to set system profile",
		append(machine.MaybeSudo(), "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", closurePath),
	)

	return errors.Wrap(err, "failed to set system profile")
}

func ActivateConfiguration(exc *executioner.Executioner, machine *machine.Machine, closurePath string, mode attributes.ActivationMode) error {
	binPath := closurePath + "/bin/switch-to-configuration"

	err := exc.Exec(
		"activate",
		"activating configuration",
		"activation failed",
		append(machine.MaybeSudo(), binPath, string(mode)),
	)

	return errors.Wrap(err, "failed to activate configuration")
}
