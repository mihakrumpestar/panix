package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mihakrumpestar/panix/internal/workflow"
)

type errMsg error

type model struct {
	//spinner  spinner.Model
	meta     *workflow.Metadatas
	quitting bool
	err      error
}

func NewTui(meta *workflow.Metadatas) error {
	p := tea.NewProgram(initialModel(meta))
	_, err := p.Run()
	if err != nil {
		return err
	}

	return nil
}

func initialModel(meta *workflow.Metadatas) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		//spinner: s,
		meta: meta,
	}
}

func (m model) Init() tea.Cmd {
	//return m.spinner.Tick
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		default:
			return m, nil
		}

	case errMsg:
		m.err = msg
		return m, nil

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
