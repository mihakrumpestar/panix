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
	meta         *workflow.Metadatas
	quitting     bool
	err          error
	updateCh     <-chan uint64
	cancelParent context.CancelFunc
}

func NewTui(meta *workflow.Metadatas, updateCh <-chan uint64, cancel context.CancelFunc) error {
	p := tea.NewProgram(initialModel(meta, updateCh, cancel))
	_, err := p.Run()
	if err != nil {
		return err
	}

	return nil
}

func initialModel(meta *workflow.Metadatas, updateCh <-chan uint64, cancel context.CancelFunc) model {
	return model{
		meta:         meta,
		updateCh:     updateCh,
		cancelParent: cancel,
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

		if m.meta.Error != nil {
			return m.meta.Error
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
		default:
			return m, nil
		}

	case errMsg:
		m.err = msg
		return m, nil

	case updateMsg:
		// Trigger re-render when update is received
		return m, m.listenForUpdates()

	default:
		var cmd tea.Cmd
		//m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m model) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	str := fmt.Sprintf("\n\n   %s Loading forever...press q to quit\n\n", nil /*m.spinner.View()*/)
	table, err := m.meta.PrintStatusPhaseMachineTable()
	if err != nil {
		//panic(err)
	} else {
		str += table.String()
	}
	if m.quitting {
		return str + "\n"
	}
	return str
}
