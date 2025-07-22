package tui

import (
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

type stateUpdateHookMsg struct{}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type TuiViewMode string

var (
	TuiViewModeAll    TuiViewMode = "all"
	TuiViewModeStatus TuiViewMode = "status"
	TuiViewModeLogs   TuiViewMode = "logs"
)

type model struct {
	workflow  *workflow.Workflow
	quitting  bool
	err       error
	modelView modelView
}

type modelView struct {
	mode        TuiViewMode
	width       int
	height      int
	spinners    *Spinners
	debugOutput strings.Builder
	colors      ColorScheme
	//viewport viewport.Model
}

func NewTui(workflow *workflow.Workflow) error {

	defaultModelView := TuiViewModeAll
	if !slices.Contains(workflow.Phases(), workflow_definition.PhaseStatus) {
		defaultModelView = TuiViewModeLogs
	}

	p := tea.NewProgram(model{
		workflow: workflow,
		modelView: modelView{mode: defaultModelView,
			spinners: NewSpinners(),
			width:    120, // Initial dimensions
			height:   120, // Initial dimensions
			colors:   DefaultColorScheme(),
		},
	},
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)
	m, err := p.Run()

	// Print the final view to stdout after exiting alt-screen
	finalModel, ok := m.(model)
	if ok {
		fmt.Println(finalModel.View())
	}

	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		m.stateUpdateHook(),
		m.startWorkflow(),
	)
}

func (m model) startWorkflow() tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				err := errors.New("stacktrace from panic: \n" + string(debug.Stack()))
				m.err = err
				//return errMsg{err}
			}
		}()

		err := m.workflow.Start()
		if err != nil {
			m.err = err
			return errMsg{err}
		}

		return errMsg{}
	}
}

func (m model) stateUpdateHook() tea.Cmd {
	return func() tea.Msg {
		_, ok := <-m.workflow.GetChannel()
		if !ok {
			return tea.Quit()
		}

		return stateUpdateHookMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	cmds = append(cmds, m.modelView.spinners.SendInitTickIfNotAlready())

	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			m.workflow.Cancel()()

			// Block quiting until Workflow finalizes and terminates its tasks
			<-m.workflow.Ctx().Done()

			return m, tea.Quit
		case "d":
			// Don't toggle if we don't have PhaseStatus
			if !slices.Contains(m.workflow.Phases(), workflow_definition.PhaseStatus) {
				return m, nil
			}

			// Toggle between status and detailed views
			switch m.modelView.mode {
			case TuiViewModeAll:
				m.modelView.mode = TuiViewModeLogs
			case TuiViewModeLogs:
				m.modelView.mode = TuiViewModeStatus
			case TuiViewModeStatus:
				fallthrough
			default:
				m.modelView.mode = TuiViewModeAll
			}
			return m, nil
		default:
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.modelView.width = msg.Width
		m.modelView.height = msg.Height

		/*
			headerHeight := lipgloss.Height(m.modelView.headerView())
			footerHeight := lipgloss.Height(m.footerView())
			verticalMarginHeight := headerHeight + footerHeight

			if !m.modelView.ready {
				// Since this program is using the full size of the viewport we
				// need to wait until we've received the window dimensions before
				// we can initialize the viewport. The initial dimensions come in
				// quickly, though asynchronously, which is why we wait for them
				// here.
				m.modelView.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
				m.modelView.viewport.YPosition = headerHeight
				m.modelView.viewport.SetContent(m.content)
				m.modelView.ready = true
			} else {
				m.modelView.viewport.Width = msg.Width
				m.modelView.viewport.Height = msg.Height - verticalMarginHeight
			}
		*/

	// Trigger re-render when update is received
	case stateUpdateHookMsg:
		cmds = append(cmds,
			m.stateUpdateHook(),
		)

	// Update spinners
	case spinner.TickMsg:
		cmds = append(cmds, m.modelView.spinners.Update(msg))
	default:
	}

	// Handle keyboard and mouse events in the viewport
	//m.modelView.viewport, cmd = m.modelView.viewport.Update(msg)
	//cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var builder strings.Builder

	// Header with instructions
	header := "\n=== Panix TUI ===\n"
	instructions := "Press 'q' to quit, 'd' to switch modes\n\n"
	headerAndInstructions := header + instructions
	builder.WriteString(headerAndInstructions)

	builder.WriteString(fmt.Sprintf("%v\n", m.workflow.Phases()))

	if m.workflow.State() == nil {
		builder.WriteString("No state available")
	}

	switch m.modelView.mode {
	case TuiViewModeAll:
		fallthrough
	case TuiViewModeStatus:
		view := m.PrintStatusPhaseMachineTable()
		if view == "" {
			builder.WriteString("No data available")
		} else {
			builder.WriteString(view)

		}

		if m.modelView.mode != TuiViewModeAll {
			break
		}

		fallthrough
	case TuiViewModeLogs:
		view := m.PrintBuildLogs()
		if view == "" {
			builder.WriteString("No data available")
		} else {
			builder.WriteString(view)
		}
	}

	if m.err != nil {
		errorHeader := "\n\n\n=== Error ===\n"
		errorContent := "\n%s\n" + m.err.Error()
		builder.WriteString(errorHeader + errorContent)
	}

	if m.workflow.State().Conf.Global.Debug {
		debugHeader := "\n\n\n=== Debug ===\n"
		debugContent := m.modelView.spinners.Debug()
		debugContent += "\nDebug console output:\n" + m.modelView.debugOutput.String()
		builder.WriteString(debugHeader + debugContent)
	}

	if m.quitting {
		builder.WriteString("\n")
		return builder.String()
	}

	return builder.String()
}
