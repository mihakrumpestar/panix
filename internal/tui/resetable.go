package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_viewports"
	"github.com/mihakrumpestar/panix/internal/workflow"
	zerolog "github.com/rs/zerolog/log"
)

type resetable struct {
	err         error
	workflow    *workflow.Workflow
	spinners    *tui_spinners.Spinners
	viewports   *tui_viewports.Viewports
	statsTable  *StatsTable
	phaseStatus *PhaseStatus
}

// workflowDoneMsg signals the workflow has completed
type workflowDoneMsg struct{}

func (m *model) startWorkflow() tea.Cmd {
	return func() tea.Msg {
		workflow, err := workflow.NewWorkflow(m.ctx, m.conf)
		if err != nil {
			return errMsg{err}
		}

		spinners, err := tui_spinners.NewSpinners()
		if err != nil {
			return errMsg{err}
		}

		m.resetable.Store(&resetable{
			workflow:    workflow,
			spinners:    spinners,
			viewports:   tui_viewports.NewViewports(m.dimensions, m.conf.ColorScheme, nil, m.conf.Flags.Tui.CommandOutputMaxHeight),
			statsTable:  NewStatsTable(),
			phaseStatus: NewPhaseStatus(),
		})

		err = workflow.CreateWorkflow()
		if err != nil {
			zerolog.Error().Err(err).Msg("Workflow execution failed")
			if err != context.Canceled {
				return errMsg{err}
			}
		} else {
			zerolog.Info().Msg("Workflow completed successfully")
		}

		return workflowDoneMsg{}
	}
}

func (m *model) restartWorkflow() tea.Cmd {
	if r := m.resetable.Load(); r != nil {
		r.workflow.Cancel()
	}

	return m.startWorkflow()
}
