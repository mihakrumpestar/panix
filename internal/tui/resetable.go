package tui

import (
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/rs/zerolog/log"
)

type workflowDoneMsg struct {
	err error
}

func (m *model) startWorkflowCmd() zeroterm.Cmd {
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

func (m *model) restartWorkflow() zeroterm.Cmd {
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

	return m.startWorkflowCmd()
}

func (m *model) retryWorkflow() {
	if m.workflow == nil {
		return
	}

	m.workflow.State().Retry.Trigger()
}
