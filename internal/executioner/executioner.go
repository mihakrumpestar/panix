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
	phaseLog     *config.Log
	onUpdateHook func()
}

// NewExecutioner: if machine == nil it indicates that the command will be executred locally
func NewExecutioner(ctx context.Context, conf *config.Global, machine *config.Machine, phaseLog *config.Log, onUpdateHook func()) *Executioner {

	return &Executioner{
		ctx:          ctx,
		dryRun:       conf.DryRun,
		machine:      machine,
		phaseLog:     phaseLog,
		onUpdateHook: onUpdateHook,
	}
}

func (ex *Executioner) Exec(skipIfLocal, log bool, onFailure func(*config.Log, error) error, onSuccess func(*config.Log) error, name string, args ...string) error {
	defer ex.onUpdateHook()

	noMachineOrLocal := ex.machine == nil || ex.machine.Ssh.IsLocal

	// 1) local short‐circuit
	if noMachineOrLocal && skipIfLocal {
		ex.phaseLog.AddMessageOnly("(skipped) ", name, " ", strings.Join(args, " "))
		if onSuccess != nil {
			err := onSuccess(ex.phaseLog)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if noMachineOrLocal {
		return ex.shellStream(log, onFailure, onSuccess, name, args...)
	} else {
		return ex.sshStream(log, onFailure, onSuccess, name, args...)
	}
}
