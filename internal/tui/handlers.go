package tui

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/rs/zerolog/log"
)

const (
	workflowUpdateHookPollInterval  = 20 * time.Millisecond
	workflowUpdateHookThrottleDelay = 40 * time.Millisecond
)

type (
	workflowUpdateHookMsg struct{}

	restartMsg struct{}
	retryMsg   struct{}
)

func restartCmd() zeroterm.Msg { return restartMsg{} }
func retryCmd() zeroterm.Msg   { return retryMsg{} }

type errMsg struct { //nolint:errname
	err error
}

func (e errMsg) Error() string {
	return e.err.Error()
}

type workflowDoneMsg struct {
	err error
}

func (m *model) workflowStartCmd() zeroterm.Cmd {
	return func() zeroterm.Msg {
		var err error

		m.workflow, err = workflow.NewWorkflow(m.ctx, m.conf)
		if err != nil {
			return errMsg{err: err}
		}

		err = m.workflow.StartWorkflow()

		return workflowDoneMsg{err: err}
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
	m.err = nil
	m.buildLogs = nil

	return m.workflowStartCmd()
}

func (m *model) workflowUpdateHookCmd() zeroterm.Cmd {
	return func() zeroterm.Msg {
		if m.workflow == nil {
			time.Sleep(workflowUpdateHookPollInterval)

			return workflowUpdateHookMsg{}
		}

		<-m.workflow.WaitForUpdate()

		now := time.Now()
		elapsed := now.Sub(m.lastWorkflowUpdate)

		if elapsed < workflowUpdateHookThrottleDelay {
			time.Sleep(workflowUpdateHookThrottleDelay - elapsed)
		}

		m.lastWorkflowUpdate = time.Now()

		return workflowUpdateHookMsg{}
	}
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
