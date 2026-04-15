package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/pkg/errors"
)

type resetable struct {
	err       error
	workflow  *workflow.Workflow
	spinners  *spinners.Spinners
	viewports *viewports.Viewports
}

// workflowDoneMsg signals the workflow has completed.
type workflowDoneMsg struct{}

func (m *model) startResetableWorkflow() tea.Cmd {
	return func() tea.Msg {
		workflow, err := workflow.NewWorkflow(m.ctx, m.conf)
		if err != nil {
			return errMsg{err}
		}

		spinners, err := spinners.NewSpinners()
		if err != nil {
			return errMsg{err}
		}

		m.resetable.Store(&resetable{
			workflow:  workflow,
			spinners:  spinners,
			viewports: viewports.NewViewports(m.dimensions, m.conf),
		})

		err = workflow.StartWorkflow()
		if err != nil {
			logFinalState(m.conf)

			return errMsg{err}
		}

		logFinalState(m.conf)

		return workflowDoneMsg{}
	}
}

func (m *model) restartWorkflow() tea.Cmd {
	r := m.resetable.Load()
	if r != nil {
		err := r.workflow.Cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			return func() tea.Msg {
				return errMsg{errors.Wrap(err, "failed to cancel workflow")}
			}
		}
	}

	m.conf.Fleet.ResetState()

	return m.startResetableWorkflow()
}
