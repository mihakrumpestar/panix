package workflow

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/rs/zerolog/log"
)

func (w *Workflow) executeTransferPhaseMachine(machine *config.Machine) error {
	return w.Phase(machine.Attributes.Xpath, phases.Transfer, machine,
		func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error {
			systemClosure := machine.ParentConfiguration.MetaBuild.SystemClosure

			err := executeTransferPhaseMachineWrapper(exc, phaseLog, machine, []string{systemClosure}, true)
			if err != nil {
				return err
			}

			return nil
		})
}

func executeTransferPhaseMachineWrapper(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog, machine *config.Machine, toTransfer []string, transferClosure bool) error {
	storeArgs := ""
	if !machine.MetaInspect.Bootstrapped.Load() && transferClosure {
		storeArgs += "?remote-store=local?root=/mnt"
	}

	activeSSH := machine.MetaInspect.ActiveSSH
	commandWithArgs := append([]string{"nix", "copy", "--to", "ssh://" + activeSSH.Hostname + storeArgs}, toTransfer...)

	err := exc.Exec("nix copy",
		"copying closure",
		"closure copy failed",
		commandWithArgs,
		executioner.SkipIfLocal(),
		executioner.DisableAutoSshCommand(),
		executioner.Env(activeSSH.MaybeSshEnvOpts()),
	)
	if err != nil {
		return err
	}

	log.Info().
		Str("machine", machine.Name).
		Strs("transferred", toTransfer).
		Msgf("Transferred %s to %s", toTransfer, machine.Name)

	return nil
}
