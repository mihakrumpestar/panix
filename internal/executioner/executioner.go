package executioner

import (
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
)

type Executioner struct {
	ctx          context.Context
	dryRun       bool
	machine      *config.Machine
	phaseLog     *logs.PhaseLog
	onUpdateHook func()
}

// NewExecutioner: if machine == nil it indicates that the command will be executed locally
func NewExecutioner(ctx context.Context, conf *config_flags.Flags, machine *config.Machine, phaseLog *logs.PhaseLog, onUpdateHook func()) *Executioner {
	return &Executioner{
		ctx:          ctx,
		dryRun:       conf.DryRun,
		machine:      machine,
		phaseLog:     phaseLog,
		onUpdateHook: onUpdateHook,
	}
}

func (ex *Executioner) Exec(skipIfLocal, log bool, onFailure func(*logs.CommandLog, error) error, onSuccess func(*logs.CommandLog) error, commandWithArgs ...string) error {
	noMachineOrLocal := ex.machine == nil || ex.machine.Attributes.Ssh.IsLocal

	// 1) local short‐circuit
	if noMachineOrLocal && skipIfLocal {
		defer ex.onUpdateHook()

		comLog := ex.phaseLog.AddMessageOnly("(skipped)")
		if onSuccess != nil {
			err := onSuccess(comLog)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if noMachineOrLocal {
		return ex.shellStream(log, onFailure, onSuccess, commandWithArgs...)
	}

	return ex.sshStream(log, onFailure, onSuccess, commandWithArgs...)
}
