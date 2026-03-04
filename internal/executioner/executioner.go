package executioner

import (
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
)

type Executioner struct {
	ctx          context.Context
	dryRun       bool
	machine      *config.Machine
	phaseLog     *logs_phase.PhaseLog
	onUpdateHook func()
}

func NewExecutioner(ctx context.Context, dryRun bool, machine *config.Machine, phaseLog *logs_phase.PhaseLog, onUpdateHook func()) *Executioner {
	return &Executioner{
		ctx:          ctx,
		dryRun:       dryRun,
		machine:      machine,
		phaseLog:     phaseLog,
		onUpdateHook: onUpdateHook,
	}
}

// Exec

type ExecOptions struct {
	skipIfLocal           bool
	disableAutoSshCommand bool
	onFailure             func(*logs_command.CommandLog, error) error
	onSuccess             func(*logs_command.CommandLog) error
	onDryRun              func()
	env                   []string
}

type ExecOption func(*ExecOptions)

func SkipIfLocal() ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.skipIfLocal = true
	}
}

func DisableAutoSshCommand() ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.disableAutoSshCommand = true
	}
}

func OnFailure(f func(*logs_command.CommandLog, error) error) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.onFailure = f
	}
}

func OnSuccess(f func(*logs_command.CommandLog) error) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.onSuccess = f
	}
}

// OnDryRun is mandatory - every command that has OnSuccess must also provide OnDryRun
func OnDryRun(f func()) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.onDryRun = f
	}
}

// Slice of "key=value" entrys
func Env(env []string) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.env = env
	}
}

func (ex *Executioner) Exec(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, opts ...ExecOption) error {
	excOpt := &ExecOptions{}
	for _, opt := range opts {
		opt(excOpt)
	}

	var isLocal bool
	if ex.machine != nil && ex.machine.MetaInspect.ActiveSSH != nil {
		isLocal = ex.machine.MetaInspect.ActiveSSH.IsLocal
	}
	noMachineOrLocal := ex.machine == nil || isLocal

	if noMachineOrLocal && excOpt.skipIfLocal {
		return nil
	}

	if noMachineOrLocal || excOpt.disableAutoSshCommand {
		return ex.shellStream(description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt)
	}

	return ex.sshStream(description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt)
}

func (ex *Executioner) ExecLocal(description, statusIfRunning, statusIfFailed string, commandWithArgs []string) error {
	return ex.Exec(description, statusIfRunning, statusIfFailed, commandWithArgs, DisableAutoSshCommand())
}

func (ex *Executioner) ExecFn(description, statusIfRunning, statusIfFailed string, fn func() error) error {
	commandLog := ex.phaseLog.NewCommand(description, statusIfRunning, statusIfFailed, nil, nil)

	commandLog.TimeAndState.StartTimer()
	var execErr error
	defer func() {
		commandLog.TimeAndState.EndTimerWithError(execErr)
		ex.onUpdateHook()
	}()

	ex.onUpdateHook()

	if ex.dryRun {
		return nil
	}

	execErr = fn()
	if execErr != nil {
		return execErr
	}
	return nil
}
