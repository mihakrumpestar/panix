package inspect

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

type Handler struct{}

//nolint:cyclop
func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machineI := fleetLeaf.Machine

	if machineI.SSH.IsLocal() {
		machineI.MetaInspect.Update(func(mi *machine.MetaInspect) {
			mi.Reachable = true
			mi.SSHConnectable = true
		})
	} else {
		err := checkSSHReachability(exc, machineI)
		if err != nil {
			return err
		}

		err = checkSSHConnection(exc, machineI)
		if err != nil {
			return err
		}
	}

	err := detectArchitecture(exc, machineI)
	if err != nil {
		return err
	}

	err = checkSuperuser(exc, machineI)
	if err != nil {
		return err
	}

	err = detectBootstrapStatus(exc, machineI)
	if err != nil {
		return err
	}

	err = validateSSHMachineState(exc, machineI)
	if err != nil {
		return err
	}

	err = validateSecretsPaths(exc, machineI)
	if err != nil {
		return err
	}

	mi := machineI.MetaInspect.Load()
	if mi != nil && !mi.Bootstrapped {
		return handleUnbootstrapped(exc, machineI)
	}

	return readGenerations(exc, machineI)
}
