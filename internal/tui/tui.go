package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mihakrumpestar/panix/internal/workflow"
)

type errMsg error

type updateMsg struct{}

type model struct {
	state        *workflow.WorkflowState
	quitting     bool
	err          error
	updateCh     <-chan uint64
	cancelParent context.CancelFunc
	viewMode     string // "status", "detailed", or "table"
	width        int    // terminal width for responsive rendering
}

func NewTui(state *workflow.WorkflowState, updateCh <-chan uint64, cancel context.CancelFunc) error {
	p := tea.NewProgram(initialModel(state, updateCh, cancel), tea.WithAltScreen())
	m, err := p.Run()

	// Print the final view to stdout after exiting alt-screen
	if finalModel, ok := m.(model); ok {
		fmt.Println(finalModel.View())
	}

	if err != nil {
		return err
	}

	return nil
}

func initialModel(state *workflow.WorkflowState, updateCh <-chan uint64, cancel context.CancelFunc) model {
	return model{
		state:        state,
		updateCh:     updateCh,
		cancelParent: cancel,
		viewMode:     "status", // Default to status view
		width:        120,      // Default width, will be updated by WindowSizeMsg
	}
}

func (m model) Init() tea.Cmd {
	//return m.spinner.Tick
	return m.listenForUpdates()
}

func (m model) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		_, ok := <-m.updateCh
		if !ok {
			return tea.Quit()
		}

		if m.state.Error != nil {
			return m.state.Error
		}

		return updateMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			m.cancelParent()
			return m, tea.Quit
		case "d":
			// Toggle between status and detailed views
			if m.viewMode == "status" {
				m.viewMode = "detailed"
			} else {
				m.viewMode = "status"
			}
			return m, nil
		case "t":
			// Toggle to phase meta table view
			m.viewMode = "table"
			return m, nil
		default:
			return m, nil
		}

	case tea.WindowSizeMsg:
		// Update terminal width for responsive rendering
		m.width = msg.Width
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case updateMsg:
		// Trigger re-render when update is received
		return m, m.listenForUpdates()

	default:
		var cmd tea.Cmd
		return m, cmd
	}
}

func (m model) View() string {

	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var str string

	// Header with instructions
	header := "\n=== Panix TUI ===\n"
	instructions := "Press 'd' to toggle detailed view, 't' for table view, 'q' to quit\n\n"

	// Create a defensive copy of metadata to avoid concurrent access issues
	state := m.state

	switch m.viewMode {
	case "detailed":
		if state == nil {
			str = header + instructions + "No metadata available"
			break
		}
		/*
			detailed, err := state.PrintDetailedPhaseMeta()
			if err != nil {
				str = fmt.Sprintf("Error: %v", err)
			} else {
				str = header + instructions + detailed
			}
		*/

	case "table":
		if state == nil {
			str = header + instructions + "No metadata available"
			break
		}
		/*
			table, err := state.PrintPhaseMetaTable()
			if err != nil {
				str = fmt.Sprintf("Error: %v", err)
			} else if table != nil {
				str = header + instructions + table.String()
			} else {
				str = header + instructions + "No phase meta data available"
			}
		*/

	case "status":
		fallthrough
	default:
		if state == nil {
			str = header + instructions + "No metadata available"
			break
		}
		table, err := state.PrintStatusPhaseMachineTable(m.width)
		if err != nil {
			str = fmt.Sprintf("Error: %v", err)
		} else if table != nil {
			str = header + instructions + table.String()
		} else {
			str = header + instructions + "No status data available"
		}
	}

	if m.quitting {
		return str + "\n"
	}
	return str
}
