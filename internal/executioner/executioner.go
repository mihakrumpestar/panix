package executioner

import (
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
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

// NewExecutioner: if machine == nil it indicates that the command will be executed locally
func NewExecutioner(ctx context.Context, conf *config_flags.Flags, machine *config.Machine, phaseLog *logs_phase.PhaseLog, onUpdateHook func()) *Executioner {
	return &Executioner{
		ctx:          ctx,
		dryRun:       conf.DryRun,
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

// Slice of "key=value" entrys
func Env(env []string) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.env = env
	}
}

func (ex *Executioner) Exec(description, statusIfFailed string, commandWithArgs []string, opts ...ExecOption) error {
	excOpt := &ExecOptions{}
	for _, opt := range opts {
		opt(excOpt)
	}

	noMachineOrLocal := ex.machine == nil || ex.machine.SSH.IsLocal

	// 1) local short‐circuit
	if noMachineOrLocal && excOpt.skipIfLocal {
		defer ex.onUpdateHook()

		comLog := ex.phaseLog.NewCommand("(skipped)", "", true)
		if excOpt.onSuccess != nil {
			err := excOpt.onSuccess(comLog)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if noMachineOrLocal || excOpt.disableAutoSshCommand {
		return ex.shellStream(description, statusIfFailed, commandWithArgs, excOpt)
	}

	return ex.sshStream(description, statusIfFailed, commandWithArgs, excOpt)
}
