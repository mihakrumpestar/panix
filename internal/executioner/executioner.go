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

// NewExecutioner: if machineName == nil, machineSshConfig won't be used
func NewExecutioner(ctx context.Context, conf *config.Global, machine *config.Machine, phaseLog *config.Log, onUpdateHook func()) *Executioner {

	return &Executioner{
		ctx:          ctx,
		dryRun:       conf.DryRun,
		machine:      machine,
		phaseLog:     phaseLog,
		onUpdateHook: onUpdateHook,
	}
}

func (ex *Executioner) Exec(skipIfLocal bool, onFailure func(*config.Log, error) error, onSuccess func(*config.Log) error, name string, args ...string) error {
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
		return ex.shellStream(onFailure, onSuccess, name, args...)
	} else {
		return ex.sshStream(onFailure, onSuccess, name, args...)
	}
}
