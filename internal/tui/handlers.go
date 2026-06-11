package tui

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/rs/zerolog/log"
)

const (
	workflowUpdateHookThrottleDelay = time.Second / 60 // FPS
)

type (
	workflowUpdateHookMsg struct {
		workflow   *workflow.Workflow
		lastUpdate time.Time
	}

	restartMsg struct{}
	retryMsg   struct{}
)

func restartCmd() zeroterm.Msg { return restartMsg{} }
func retryCmd() zeroterm.Msg   { return retryMsg{} }

type errMsg struct { //nolint:errname
	err error
}

func errCmd(err error) zeroterm.Cmd { return func() zeroterm.Msg { return errMsg{err: err} } }

func (e errMsg) Error() string {
	return e.err.Error()
}

type workflowDoneMsg struct {
	workflow *workflow.Workflow
	err      error
}

func workflowRunCmd(w *workflow.Workflow) zeroterm.Cmd {
	return func() zeroterm.Msg {
		err := w.StartWorkflow()

		return workflowDoneMsg{workflow: w, err: err}
	}
}

func workflowUpdateHookCmd(workflow *workflow.Workflow, lastUpdate time.Time) zeroterm.Cmd {
	return func() zeroterm.Msg {
		<-workflow.WaitForUpdate()

		now := time.Now()
		elapsed := now.Sub(lastUpdate)

		if elapsed < workflowUpdateHookThrottleDelay {
			time.Sleep(workflowUpdateHookThrottleDelay - elapsed)
		}

		return workflowUpdateHookMsg{workflow: workflow, lastUpdate: now}
	}
}

// workflowRestartCmd is not async.
func (m *model) workflowRestartCmd() zeroterm.Cmd {
	if m.workflow != nil {
		err := m.workflow.Cancel()
		if err != nil {
			log.Debug().Err(err).Msg("workflow cancelled for restart")
		}
	}

	m.conf.Fleet.ResetState()
	m.statsTable.Reset()
	m.phaseFlow.Reset()
	m.spinners.Reset()
	m.viewports.Reset()
	m.cachedTree.Reset()
	m.err = nil

	w, err := workflow.NewWorkflow(m.ctx, m.conf)
	if err != nil {
		return errCmd(err)
	}

	m.workflow = w

	return zeroterm.BatchCmd(
		workflowRunCmd(m.workflow),
		workflowUpdateHookCmd(m.workflow, m.lastWorkflowUpdate),
	)
}

// workflowDoneMsgCmd is not async.
func (m *model) workflowDoneMsgCmd(msg workflowDoneMsg) zeroterm.Cmd {
	if msg.err != nil {
		m.err = msg.err
		log.Error().Err(msg.err).Msg("workflowDoneMsg error")
	}

	if m.isSnapshot {
		return nil
	}

	log.Debug().Msg("workflowDoneMsg")

	m.conf.Fleet.Recalculate(m.conf.Phases)
	logFinalState(m.conf)

	if m.conf.Flags.Snapshot.OnExit {
		m.captureSnapshot(config.SnaphsotReasonExit)
	}

	if m.conf.Flags.ExitOnComplete {
		m.quitting = true

		return zeroterm.QuitCmd
	}

	return nil
}

// workflowRetry is not async.
func (m *model) workflowRetry() {
	if m.workflow == nil {
		return
	}

	m.workflow.State().Retry.Trigger()
}

//

func (m *model) handleErrMsgCmd(msg errMsg) zeroterm.Cmd {
	m.err = msg.err

	m.conf.Fleet.Recalculate(m.conf.Phases)
	logFinalState(m.conf)

	log.Error().Err(msg.err).Msg("errMsg")

	m.quitting = true

	return zeroterm.QuitCmd
}

func (m *model) handleMouseClick(msg zeroterm.MouseClickMsg) {
	if m.statsTable.HandleMouseClick(msg) {
		m.phaseFlow.Reset()
	}

	if m.phaseFlow.HandleMouseClick(msg) {
		m.statsTable.Reset()
	}
}
