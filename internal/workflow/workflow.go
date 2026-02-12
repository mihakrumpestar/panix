package workflow

import (
	"context"

	"github.com/alitto/pond/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/pkg/hook"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/retry"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/rs/zerolog/log"
)

type Workflow struct {
	ctx        context.Context
	cancel     context.CancelFunc
	state      *WorkflowState
	updateHook *hook.Hook
}

type WorkflowState struct {
	Conf   *config.Config
	Phases []phases.Phase
	Pool   pond.Pool
	Retry  *retry.Retry
}

func NewWorkflow(ctx context.Context, conf *config.Config, phasesI []phases.Phase) (*Workflow, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, conf.Flags.Timeout)

	phases, err := phases.ValidatePhases(phasesI, conf.Flags.SkipPhases)
	if err != nil {
		cancel()
		return nil, err
	}

	return &Workflow{
		ctx:    ctxWithTimeout,
		cancel: cancel,
		state: &WorkflowState{
			Conf:   conf,
			Phases: phases,
			Pool:   pond.NewPool(0, pond.WithContext(ctxWithTimeout)),
			Retry:  retry.NewTaskRetry(),
		},
		updateHook: hook.NewHook(),
	}, nil
}

func (w *Workflow) Ctx() context.Context {
	return w.ctx
}

func (w *Workflow) State() *WorkflowState {
	return w.state
}

func (w *Workflow) WaitForUpdate() <-chan struct{} {
	return w.updateHook.WaitForUpdate()
}

func (w *Workflow) Cancel() context.CancelFunc {
	return w.cancel
}

func (w *Workflow) NewTaskWithRetry(phase phases.Phase, xpath config_attributes.Xpath, f func() error) error {
	for {
		err := f()
		if err != nil {
			if w.state.Conf.Flags.RequireAllSuccess {
				w.cancel()
				return err
			}

			w.state.Retry.Wait()
			w.state.Conf.TargetsLogs.Get(xpath).PhaseLogs.Get(phase).Clear()
		} else {
			return nil
		}
	}
}

func (w *Workflow) Phase(xpath config_attributes.Xpath, phase phases.Phase, machine *config.Machine, phaseCode func(exc *executioner.Executioner, phaseLog *logs_phase.PhaseLog) error) (err error) {
	phaseLog := w.state.Conf.TargetsLogs.GetOrCreateLog(xpath, phase)

	phaseLog.TimeAndState().StartTimer()
	defer func() {
		phaseLog.TimeAndState().EndTimerWithError(err)
	}()

	log.Info().
		Str("phase", string(phaseLog.Phase())).
		Str("xpath", xpath.String()).
		Msgf("Started %s of %s", phaseLog.Phase(), xpath)

	exc := executioner.NewExecutioner(w.ctx, w.state.Conf.Flags, machine, phaseLog, w.updateHook.Signal)
	err = phaseCode(exc, phaseLog)

	log.Info().
		Str("phase", string(phaseLog.Phase())).
		Str("xpath", xpath.String()).
		Msgf("Finished %s of %s", phaseLog.Phase(), xpath)

	return err
}

// CreateWorkflow orchestrates the execution of all phases
func (w *Workflow) CreateWorkflow() error {
	subPool := w.state.Pool.NewGroup()

	w.state.RootTree(func(i int, machine *config.Machine) {
		subPool.SubmitErr(func() error {
			// Create a shared phase runner for this machine
			runner := &phaseRunner{
				w:       w,
				flake:   machine.Configuration.Flake,
				config:  machine.Configuration,
				machine: machine,
			}

			// Execute each phase in order
			for _, phase := range w.state.Phases {
				if err := runner.run(phase); err != nil {
					return err
				}
			}

			return nil
		})
	})

	err := subPool.Wait()
	w.updateHook.Close()
	return err
}

func (w *WorkflowState) RootTree(function func(i int, machine *config.Machine)) {
	i := 0

	for _, flakePair := range w.Conf.Root.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		for _, configPair := range flake.Configurations.Omap.Pairs() {
			configuration := configPair.Value
			for _, machinePair := range configuration.Machines.Omap.Pairs() {
				machine := machinePair.Value
				function(i, machine)
				i++
			}
		}
	}
}
