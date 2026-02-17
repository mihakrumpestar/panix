package workflow

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func (w *Workflow) executeBootstrapPhaseMachine(flake *config.Flake, configuration *config.Configuration, machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Bootstrap, machine,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			installables := []string{fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.diskoScript", flake.URL, configuration.Name)}

			parsedOutput, err := w.executeBuildPhaseConfigurationWrapper(exc, phaseLog, flake, configuration, installables, "disko")
			if err != nil {
				return err
			}

			diskoScript := parsedOutput[0].Outputs.Out

			err = executeTransferPhaseMachineWrapper(exc, phaseLog, machine, []string{diskoScript}, false)
			if err != nil {
				return err
			}

			// Upload disk encryption keys BEFORE running disko
			// Keys must be available for LUKS unlocking during partitioning
			if len(machine.Bootstrap.DiskEncryptionKeys) > 0 {
				if err := w.executeDiskEncryptionKeys(exc, machine, phaseLog); err != nil {
					return err
				}
			}

			err = exc.Exec(
				"disko",
				"partitioning disk",
				"diskoScript failed",
				[]string{diskoScript},
			)
			if err != nil {
				return err
			}

			if machine.Bootstrap.PostBootstrapHook == "" {
				return nil
			}

			err = exc.Exec(
				"post bootstrap hook",
				"running post-bootstrap hook",
				"post bootstrap hook failed",
				[]string{machine.Bootstrap.PostBootstrapHook},
			)
			if err != nil {
				return err
			}

			return nil
		},
	)
}

// executeDiskEncryptionKeys transfers disk encryption keys to the target machine
// Must be called BEFORE disko runs, so keys are available for LUKS unlocking
func (w *Workflow) executeDiskEncryptionKeys(
	exc *executioner.Executioner,
	machine *config.Machine,
	phaseLog *logs_phase.PhaseLog,
) error {
	for _, diskEncryptionKey := range machine.Bootstrap.DiskEncryptionKeys {
		err := w.transferPlainFileOrDir(exc, machine, diskEncryptionKey, "disk encryption key")
		if err != nil {
			return errors.Wrapf(err, "failed to transfer disk encryption key to %s", diskEncryptionKey.RemotePath)
		}
	}

	return nil
}
