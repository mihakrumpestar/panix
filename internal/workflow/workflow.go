package workflow

import (
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/activate"
	"github.com/mihakrumpestar/panix/internal/workflow/bootstrap"
	"github.com/mihakrumpestar/panix/internal/workflow/build"
	"github.com/mihakrumpestar/panix/internal/workflow/inspect"
	"github.com/mihakrumpestar/panix/internal/workflow/phasehandler"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/internal/workflow/rollback"
	"github.com/mihakrumpestar/panix/internal/workflow/secrets"
	"github.com/mihakrumpestar/panix/internal/workflow/transfer"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/hook"
	"github.com/mihakrumpestar/panix/pkg/onceasync"
	"github.com/mihakrumpestar/panix/pkg/pool"
	"github.com/mihakrumpestar/panix/pkg/retry"
	"github.com/pkg/errors"
)

type Workflow struct {
	parentCtx context.Context
	ctx       context.Context

	cancel     context.CancelFunc
	conf       *config.Config
	state      *WorkflowState
	updateHook *hook.Hook
	done       chan struct{}

	handlers     map[phase.Phase]phasehandler.Handler
	onceRegistry *atomicorderedmap.AtomicOrderedMap[string, *onceasync.OnceAsync]
	groupCtx     context.Context
}

type WorkflowState struct {
	Pool  *pool.Pool
	Retry *retry.Retry
}

func NewWorkflow(ctx context.Context, conf *config.Config) (*Workflow, error) {
	ctxWithCancel, cancel := context.WithCancel(ctx)

	outLinks := phaseops.OutLinks{
		Enabled: conf.Flags.OutLinks,
		Dir:     conf.Flags.OutLinksDir,
	}

	workflow := &Workflow{
		parentCtx: ctx,
		ctx:       ctxWithCancel,

		cancel: cancel,
		conf:   conf,
		state: &WorkflowState{
			Pool:  pool.New(conf.Flags.Runtime.MaxConcurrency, ctxWithCancel),
			Retry: retry.NewTaskRetry(),
		},
		updateHook: hook.NewHook(),
		done:       make(chan struct{}),

		handlers: map[phase.Phase]phasehandler.Handler{
			phase.Inspect:   inspect.Handler{},
			phase.Build:     build.Handler{OutLinks: outLinks},
			phase.Bootstrap: bootstrap.Handler{OutLinks: outLinks},
			phase.Transfer:  transfer.Handler{},
			phase.Secrets:   secrets.Handler{},
			phase.Activate: activate.Handler{
				ActivationMode: conf.Flags.ActivationMode,
				NixFlavor:      conf.Nix.GetFlavor(),
			},
			phase.Rollback: rollback.Handler{
				TargetGeneration: conf.Flags.Generation,
				NixFlavor:        conf.Nix.GetFlavor(),
			},
		},
		onceRegistry: atomicorderedmap.New[string, *onceasync.OnceAsync](),
	}

	return workflow, nil
}

func (w *Workflow) State() *WorkflowState {
	return w.state
}

func (w *Workflow) WaitForUpdate() <-chan struct{} {
	return w.updateHook.WaitForUpdate()
}

func (w *Workflow) Cancel() error {
	w.cancel()

	if w.done != nil {
		<-w.done
	}

	w.updateHook.Close()

	return errors.Wrap(w.ctx.Err(), "context error")
}

// CancelAsync cancels the workflow context without blocking on workflow
// completion. The workflow finishes asynchronously; StartWorkflow's deferred
// cleanup closes the update hook and the done channel once it does, so callers
// that need to know when cancellation finished should wait on Done().
func (w *Workflow) CancelAsync() {
	w.cancel()
}

// Done returns a channel that is closed once the workflow has fully finished
// (StartWorkflow returned).
func (w *Workflow) Done() <-chan struct{} {
	return w.done
}

func (w *Workflow) MachineCount() int {
	count := 0

	for range w.conf.Fleet.AllMachines() {
		count++
	}

	return count
}
