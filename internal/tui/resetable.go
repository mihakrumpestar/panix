package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/rs/zerolog/log"
)

type resetable struct {
	err         error
	workflow    *workflow.Workflow
	spinners    *spinners.Spinners
	viewports   *viewports.Viewports
	statsTable  *StatsTable
	phaseStatus *PhaseStatus
}

// workflowDoneMsg signals the workflow has completed.
type workflowDoneMsg struct{}

func (m *model) startWorkflow() tea.Cmd {
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
			workflow:    workflow,
			spinners:    spinners,
			viewports:   viewports.NewViewports(m.dimensions, m.conf),
			statsTable:  NewStatsTable(),
			phaseStatus: NewPhaseStatus(),
		})

		err = workflow.CreateWorkflow()
		if err != nil {
			log.Error().Err(err).Msg("Workflow execution failed")

			return errMsg{err}
		} else {
			log.Info().Msg("Workflow completed successfully")
		}

		return workflowDoneMsg{}
	}
}

func (m *model) restartWorkflow() tea.Cmd {
	if r := m.resetable.Load(); r != nil {
		err := r.workflow.Cancel()
		if err != nil {
			log.Error().Err(err).Msg("failed to cancel workflow")
		}
	}

	return m.startWorkflow()
}
