package executioner

import (
	"context"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
)

type Executioner struct {
	ctx          context.Context
	dryRun       bool
	machine      *config.Machine
	phaseLog     *config.PhaseLog
	onUpdateHook func()
}

// NewExecutioner: if machine == nil it indicates that the command will be executed locally
func NewExecutioner(ctx context.Context, conf *config.Global, machine *config.Machine, phaseLog *config.PhaseLog, onUpdateHook func()) *Executioner {

	return &Executioner{
		ctx:          ctx,
		dryRun:       conf.DryRun,
		machine:      machine,
		phaseLog:     phaseLog,
		onUpdateHook: onUpdateHook,
	}
}

func (ex *Executioner) Exec(skipIfLocal, log bool, onFailure func(*config.CommandLog, error) error, onSuccess func(*config.CommandLog) error, commandWithArgs ...string) error {
	defer ex.onUpdateHook()

	noMachineOrLocal := ex.machine == nil || ex.machine.Ssh.IsLocal

	// 1) local short‐circuit
	if noMachineOrLocal && skipIfLocal {
		comLog := ex.phaseLog.AddMessageOnly("(skipped) ", strings.Join(commandWithArgs, " "))
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
	} else {
		return ex.sshStream(log, onFailure, onSuccess, commandWithArgs...)
	}
}
