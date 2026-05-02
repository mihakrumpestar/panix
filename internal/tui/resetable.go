package tui

import (
	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/rs/zerolog/log"
)

type resetable struct {
	workflow  *workflow.Workflow
	viewports *viewports.Viewports
}

type workflowDoneMsg struct {
	err error
}

func (m *model) startResetableWorkflow() render.Cmd {
	return func() render.Msg {
		workflow, err := workflow.NewWorkflow(m.ctx, m.conf)
		if err != nil {
			return errMsg{err: err}
		}

		m.resetable.Store(&resetable{
			workflow:  workflow,
			viewports: viewports.NewViewports(m.dimensions, m.conf),
		})

		err = workflow.StartWorkflow()

		return workflowDoneMsg{err: err}
	}
}

func (m *model) restartWorkflow() render.Cmd {
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
