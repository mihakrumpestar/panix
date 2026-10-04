package phaseops

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/pkg/errors"
)

// ProbeNixAvailable is the single `nix --version` probe against the target and
// the single writer of MetaInspect.NixAvailable. A failed probe is not
// swallowed: callers decide whether a missing nix is fatal (custom output
// types), tolerated (nix-install bootstrap) or expected (verification).
func ProbeNixAvailable(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.Exec(
		"nix check",
		"checking nix availability",
		"nix not found on remote machine",
		[]string{"nix", "--version"},
		executioner.OnSuccess(func(_ *command.CommandLog) error {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.NixAvailable = true
			})

			return nil
		}),
		executioner.OnFailure(func(_ *command.CommandLog, err error) error {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.NixAvailable = false
			})

			return err
		}),
		executioner.OnDryRun(func() {
			machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
				mi.NixAvailable = true
			})
		}),
	)

	return errors.Wrap(err, "nix availability check failed")
}
