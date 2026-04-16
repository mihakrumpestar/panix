package executioner

import (
	"context"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logger"
	logs_command "github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	log_sphase "github.com/mihakrumpestar/panix/internal/pkg/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Executioner struct {
	ctx          context.Context
	timeout      time.Duration
	dryRun       bool
	xpath        xpath.Xpath
	machine      *machine.Machine
	phase        phase.Phase
	phaseLog     *log_sphase.PhaseLog
	onUpdateHook func()
}

func NewExecutioner(
	ctx context.Context,
	timeout time.Duration,
	dryRun bool,
	xpath xpath.Xpath,
	machine *machine.Machine,
	phase phase.Phase,
	phaseLog *log_sphase.PhaseLog,
	onUpdateHook func(),
) *Executioner {
	return &Executioner{
		ctx:          ctx,
		timeout:      timeout,
		dryRun:       dryRun,
		xpath:        xpath,
		machine:      machine,
		phase:        phase,
		phaseLog:     phaseLog,
		onUpdateHook: onUpdateHook,
	}
}

// Exec

type ExecOptions struct {
	skipIfLocal           bool
	disableAutoSSHCommand bool
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

func DisableAutoSSHCommand() ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.disableAutoSSHCommand = true
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

// OnDryRun is mandatory - every command that has OnSuccess must also provide OnDryRun.
func OnDryRun(f func()) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.onDryRun = f
	}
}

// Env takes a slice of "key=value" entrys.
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

	if ex.machine != nil {
		if ssh := ex.machine.GetActiveSSH(); ssh != nil {
			isLocal = ssh.IsLocal
		}
	}

	noMachineOrLocal := ex.machine == nil || isLocal
	if noMachineOrLocal && excOpt.skipIfLocal {
		return nil
	}

	if noMachineOrLocal || excOpt.disableAutoSSHCommand {
		return ex.shellStream(description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt)
	}

	return ex.sshStream(description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt)
}

func (ex *Executioner) ExecFn(description, statusIfRunning, statusIfFailed string, execFunc func(*logs_command.CommandLog) error) error {
	commandLog := ex.phaseLog.NewCommand(description, statusIfRunning, statusIfFailed, nil, nil)

	endLog := ex.startCommandLog(commandLog, description, statusIfRunning, "")

	var execErr error

	defer func() {
		endLog(execErr, commandLog)
	}()

	if ex.dryRun {
		return nil
	}

	execErr = execFunc(commandLog)

	return execErr
}

func (ex *Executioner) startCommandLog(
	commandLog *logs_command.CommandLog,
	description,
	statusIfRunning,
	command string,
) func(error, *logs_command.CommandLog) {
	commandLog.TimeAndState.StartTimer()

	ctx := log.With().
		Str("xpath", ex.xpath.String()).
		Any("phase", ex.phase).
		Str("description", description)
	if command != "" {
		ctx = ctx.Str("command", command)
	}

	sublog := ctx.Logger()

	sublog.Info().Str("event", "command_start").Str("status_running", statusIfRunning).Msg("command started")

	return func(err error, commandLog *logs_command.CommandLog) {
		commandLog.TimeAndState.EndTimerWithError(err)
		duration, _ := commandLog.TimeAndState.Load().Duration()

		logger.ResultEvent(sublog, "command finished", err, func(event *zerolog.Event) {
			event.Str("event", "command_end").Dur("duration", duration).
				Str("output", string(CleanAnsiAndSpace(commandLog.Bytes())))
		})

		ex.onUpdateHook()
	}
}
