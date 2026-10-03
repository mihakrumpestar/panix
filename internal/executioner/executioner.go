package executioner

import (
	"context"
	"io"
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
	Timeouts       ExecutionerTimeouts
}

// ExecutionerTimeouts groups the operational timeout tunables used by
// disconnect/reconnect wait loops and SSH reachability checks.
type ExecutionerTimeouts struct {
	Disconnect      time.Duration
	Reconnect       time.Duration
	SSHReachability time.Duration
}

func NewExecutioner(conf ExecutionerConf) *Executioner {
	return &Executioner{
		conf:       conf,
		phaseXpath: conf.Xpath.NewXpathWithAppend(string(conf.Phase)),
	}
}

// Clone returns a new Executioner with the same configuration, for flows that
// issue execs from their own goroutine (the activation guard's confirm loop):
// per-instance state (the phase xpath) must not be shared across goroutines,
// while the phase log underneath is append-safe.
func (ex *Executioner) Clone() *Executioner {
	return &Executioner{
		conf:       ex.conf,
		phaseXpath: ex.conf.Xpath.NewXpathWithAppend(string(ex.conf.Phase)),
	}
}

// Context returns the executioner's phase context. Long-running flows select
// on its Done channel and probe Err for cancellation reporting (the
// consolidated exec error can lose the cancellation cause).
func (ex *Executioner) Context() context.Context {
	if ex.conf.Ctx == nil {
		return context.Background()
	}

	return ex.conf.Ctx
}

// DryRun reports whether this executioner only records commands without
// running them. Resolution steps that would fail on missing build artifacts
// (the guardian embed) skip themselves in dry-run so the preview still renders.
func (ex *Executioner) DryRun() bool {
	return ex.conf.DryRun
}

// SSHReachabilityTimeout returns the configured SSH reachability check timeout.
func (ex *Executioner) SSHReachabilityTimeout() time.Duration {
	return ex.conf.Timeouts.SSHReachability
}

// Exec

type ExecOptions struct {
	skipIfLocal           bool
	disableAutoSSHCommand bool
	trim                  bool
	timeout               time.Duration
	quiet                 bool
	freshConnection       bool
	stdin                 io.Reader
	outputTap             func([]byte)
	onFailure             func(*logs_command.CommandLog, error) error
	onSuccess             func(*logs_command.CommandLog) error
	onDryRun              func()
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

// OnDryRun is mandatory: every command that has OnSuccess must also provide OnDryRun.
func OnDryRun(f func()) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.onDryRun = f
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

// WithTimeout overrides the executioner-wide timeout for this single command.
// The zero value keeps the configured ExecutionerConf.Timeout.
func WithTimeout(d time.Duration) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.timeout = d
	}
}

// Quiet skips the phase-log registration for this command so it never
// renders in the TUI or the phase logs. Output capture, hooks and exit
// handling stay fully functional through an unregistered command log.
// Use for control channel execs (confirm, attach, poll) that would
// otherwise flood the narrative.
func Quiet() ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.quiet = true
	}
}

// FreshConnection forces the remote transport onto a dedicated connection
// instead of the multiplexed master: -o ControlMaster=no -o ControlPath=none
// are appended after the client arguments so they override them, because ssh
// honors the last value of a repeated option. Local execs are unaffected.
func FreshConnection() ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.freshConnection = true
	}
}

// WithStdin streams r into the PTY master of the exec for its whole lifetime,
// turning the exec into a duplex control channel: the command's stdin (the
// ssh child's stdin on remote machines) receives every byte in stream order
// through the PTY write path.
//
// The pump starts only after the PTY session is up, and stops when the exec
// ends, the context is canceled, r ends (io.EOF or any read error) or a
// master write fails. It is best-effort transport: pump failures never affect
// the exec outcome, its hooks or its log. Exec does not wait for the pump, so
// the pump may briefly outlive the exec call and exits as soon as its reader
// unblocks.
//
// r should support SetReadDeadline (os.Pipe files, net conns) or never block
// indefinitely (in-memory readers): on exec end the pump unblocks a pending
// Read with an expired deadline, and that expired deadline stays set on r
// afterward, so a reused reader must clear it first. A reader that can block
// forever and supports no deadline (io.Pipe) must be closed by its owner to
// release the pump.
//
// The PTY line discipline stays in canonical mode: stdin frames must be
// newline-delimited and shorter than the terminal canonical line buffer, and
// because a PTY swallows end-of-stream, ending r does not end the remote
// command; framing the shutdown is the protocol's responsibility.
//
// On remote machines the exec requests no remote PTY (no -t/-tt), so
// passwordless elevation is a prerequisite for stdin-driven commands.
func WithStdin(r io.Reader) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.stdin = r
	}
}

// WithOutputTap observes every raw chunk the PTY read loop delivers, in
// stream order, before terminal processing reshapes it. It turns the exec's
// output side into an event stream for protocol framing on top of the same
// exec.
//
// The callback runs synchronously on the read goroutine and must not block:
// callers buffer what they need and return. Each chunk is a fresh copy the
// callback may retain. The tap never touches the CommandLog, the TUI
// rendering, the OnUpdateHook cadence, Trim or the exit hooks. When the
// option is unset, the exec behaves exactly as without it.
func WithOutputTap(tap func(chunk []byte)) ExecOption {
	return func(excOpt *ExecOptions) {
		excOpt.outputTap = tap
	}
}

// execTimeout resolves the per-command timeout: the WithTimeout override
// when set, otherwise the executioner-wide timeout.
func (ex *Executioner) execTimeout(excOpt *ExecOptions) time.Duration {
	if excOpt.timeout != 0 {
		return excOpt.timeout
	}

	return ex.conf.Timeout
}

// newCommandLog builds the command log for one exec. Registered logs go
// through the PhaseLog so they render in the TUI; quiet logs stay
// unregistered while output capture and the exit hooks keep working
// unchanged for the caller.
func (ex *Executioner) newCommandLog(
	excOpt *ExecOptions,
	description, statusIfRunning, statusIfFailed string,
	commandWithArgs []string,
	maxOutputLines uint64,
) *logs_command.CommandLog {
	if excOpt.quiet {
		commandLog := logs_command.NewCommandLog(
			ex.phaseXpath, description, statusIfRunning, statusIfFailed, commandWithArgs,
		)
		commandLog.Output.SetMaxLines(maxOutputLines)

		return commandLog
	}

	return ex.conf.PhaseLog.NewCommand(
		ex.phaseXpath, description, statusIfRunning, statusIfFailed,
		commandWithArgs, maxOutputLines,
	)
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
	commandLog := ex.conf.PhaseLog.NewCommand(ex.phaseXpath, description, statusIfRunning, statusIfFailed, nil, 0)

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
