package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/rs/zerolog/log"
)

type resetable struct {
	workflow  *workflow.Workflow
	viewports *viewports.Viewports
}

// workflowDoneMsg signals the workflow has completed, with an optional error for non-fatal failures.
type workflowDoneMsg struct {
	err error
}

func (m *model) startResetableWorkflow() tea.Cmd {
	return func() tea.Msg {
		workflow, err := workflow.NewWorkflow(m.ctx, m.conf)
		if err != nil {
			return errMsg{err: err}
		}

		m.resetable.Store(&resetable{
			workflow:  workflow,
			viewports: viewports.NewViewports(m.dimensions, m.conf),
		})

		err = workflow.StartWorkflow()

		// Non-fatal workflow errors (e.g. "N machines failed") are included in
		// workflowDoneMsg so the TUI stays open for retry/restart.
		return workflowDoneMsg{err: err}
	}
}

func (m *model) restartWorkflow() tea.Cmd {
	r := m.resetable.Load()
	if r != nil {
		err := r.workflow.Cancel()
		if err != nil {
			log.Debug().Err(err).Msg("workflow cancelled for restart")
		}
	}

	m.conf.Fleet.ResetState()
	m.err = nil

	return m.startResetableWorkflow()
}
