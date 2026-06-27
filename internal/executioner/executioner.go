package executioner

import (
	"context"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logger"
	logs_command "github.com/mihakrumpestar/panix/internal/logs/command"
	log_sphase "github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Executioner struct {
	conf       ExecutionerConf
	phaseXpath xpath.Xpath
}

type ExecutionerConf struct {
	Ctx            context.Context
	Timeout        time.Duration
	DryRun         bool
	Xpath          xpath.Xpath
	Machine        *machine.Machine
	Phase          phase.Phase
	PhaseLog       *log_sphase.PhaseLog
	OnUpdateHook   func()
	MaxOutputLines uint64
}

func NewExecutioner(conf ExecutionerConf) *Executioner {
	return &Executioner{
		conf:       conf,
		phaseXpath: conf.Xpath.NewXpathWithAppend(string(conf.Phase)),
	}
}

// Exec

type ExecOptions struct {
	skipIfLocal           bool
	disableAutoSSHCommand bool
	trim                  bool
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

// Trim enables output trimming for this command using the configured
// max output lines. Use for commands that can produce unbounded output
// (e.g. nix copy, nix build).
func Trim() ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.trim = true
	}
}

func (ex *Executioner) Exec(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, opts ...ExecOption) error {
	excOpt := &ExecOptions{}
	for _, opt := range opts {
		opt(excOpt)
	}

	var isLocal bool

	machine := ex.conf.Machine
	if machine != nil {
		isLocal = machine.GetActiveSSH().IsLocal()
	}

	noMachineOrLocal := machine == nil || isLocal
	if noMachineOrLocal && excOpt.skipIfLocal {
		return nil
	}

	if noMachineOrLocal || excOpt.disableAutoSSHCommand {
		return ex.shellStream(description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt)
	}

	return ex.sshStream(description, statusIfRunning, statusIfFailed, commandWithArgs, excOpt)
}

func (ex *Executioner) ExecFn(description, statusIfRunning, statusIfFailed string, execFunc func(*logs_command.CommandLog) error) error {
	commandLog := ex.conf.PhaseLog.NewCommand(ex.phaseXpath, description, statusIfRunning, statusIfFailed, nil, nil, 0)

	endLog := ex.startCommandLog(commandLog, description, statusIfRunning, nil)

	var execErr error

	defer func() {
		endLog(execErr, commandLog)
	}()

	if ex.conf.DryRun {
		return nil
	}

	execErr = execFunc(commandLog)

	return execErr
}

func (ex *Executioner) startCommandLog(
	commandLog *logs_command.CommandLog,
	description,
	statusIfRunning string,
	command *buffer.LineBuf,
) func(error, *logs_command.CommandLog) {
	commandLog.TimeAndState.StartTimer()

	ctx := log.With().
		Str("xpath", ex.conf.Xpath.String()).
		Any("phase", ex.conf.Phase).
		Str("description", description)
	if command.Len() > 0 {
		ctx = ctx.Str("command", command.String())
	}

	sublog := ctx.Logger()

	sublog.Info().Str("event", "command_start").Str("status_running", statusIfRunning).Msg("command started")

	return func(err error, commandLog *logs_command.CommandLog) {
		commandLog.TimeAndState.EndTimerWithError(err)
		duration, _ := commandLog.TimeAndState.Load().Duration()

		logger.ResultEvent(sublog, "command finished", err, func(event *zerolog.Event) {
			event.Str("event", "command_end").Dur("duration", duration).
				Str("output", string(style.StripANSI(commandLog.Output.Bytes())))
		})

		ex.conf.OnUpdateHook()
	}
}
