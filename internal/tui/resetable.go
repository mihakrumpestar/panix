package tui

import (
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/rs/zerolog/log"
)

type resetable struct {
	workflow *workflow.Workflow
}

type workflowDoneMsg struct {
	err error
}

func (m *model) startResetableWorkflow() zeroterm.Cmd {
	return func() zeroterm.Msg {
		workflow, err := workflow.NewWorkflow(m.ctx, m.conf)
		if err != nil {
			return errMsg{err: err}
		}

		m.resetable.Store(&resetable{
			workflow: workflow,
		})

		err = workflow.StartWorkflow()

		return workflowDoneMsg{err: err}
	}
}

func (m *model) restartWorkflow() zeroterm.Cmd {
	r := m.resetable.Load()
	if r != nil {
		err := r.workflow.Cancel()
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

	return m.startResetableWorkflow()
}
