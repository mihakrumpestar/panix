package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mihakrumpestar/panix/internal/workflow"
)

type stateUpdateHookMsg struct{}

type TuiViewMode string

var (
	TuiViewModeAll    TuiViewMode = "all"
	TuiViewModeStatus TuiViewMode = "status"
	TuiViewModeLogs   TuiViewMode = "logs"
)

type model struct {
	ctx          context.Context
	state        *workflow.WorkflowState
	quitting     bool
	err          error
	updateCh     <-chan uint64
	cancelParent context.CancelFunc
	modelView    modelView
}

type modelView struct {
	mode        TuiViewMode
	width       int
	height      int
	spinners    *Spinners
	debugOutput strings.Builder
	//viewport viewport.Model
}

func NewTui(ctx context.Context, state *workflow.WorkflowState, updateCh <-chan uint64, cancel context.CancelFunc) error {
	p := tea.NewProgram(model{
		ctx:          ctx,
		state:        state,
		updateCh:     updateCh,
		cancelParent: cancel,
		modelView: modelView{mode: TuiViewModeAll,
			spinners: NewSpinners(),
			width:    120,
			height:   120,
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
		//m.modelView.spinner.Tick,
	)
}

func (m model) stateUpdateHook() tea.Cmd {
	return func() tea.Msg {
		_, ok := <-m.updateCh
		if !ok {
			return tea.Quit()
		}

		if m.state.Error != nil {
			return m.state.Error
		}

		return stateUpdateHookMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			m.cancelParent()

			// Block quiting until Workflow finalizes and terminates its tasks
			<-m.ctx.Done()

			return m, tea.Quit
		case "d":
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
			m.modelView.spinners.TickAndClean(msg),
		)

	// Update spinners
	case spinner.TickMsg:
		cmds = append(cmds,
			m.modelView.spinners.UpdateAndClean(msg),
		)
	}

	// Handle keyboard and mouse events in the viewport
	//m.modelView.viewport, cmd = m.modelView.viewport.Update(msg)
	//cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	m.modelView.spinners.BeforeViewConstructionHook()

	var builder strings.Builder

	// Header with instructions
	header := "\n=== Panix TUI ===\n"
	instructions := "Press 'q' to quit, 'd' to switch modes\n\n"
	headerAndInstructions := header + instructions
	builder.WriteString(headerAndInstructions)

	// Create a defensive copy of metadata to avoid concurrent access issues

	if m.state == nil {
		builder.WriteString("No state available")
		return builder.String()
	}

	switch m.modelView.mode {
	case TuiViewModeAll:
		fallthrough
	case TuiViewModeStatus:
		view := m.PrintStatusPhaseMachineTable()
		if view == "" {
			builder.WriteString("No data available")
			return builder.String()
		}

		builder.WriteString(view)

		if m.modelView.mode != TuiViewModeAll {
			break
		}

		fallthrough
	case TuiViewModeLogs:
		view := m.PrintBuildLogs()
		if view == "" {
			builder.WriteString("No data available")
			return builder.String()
		}

		builder.WriteString(view)
	}

	if m.quitting {
		builder.WriteString("\n")
		return builder.String()
	}

	if m.state.Conf.Global.Debug {
		debugHeader := "\n\n\n=== Debug ===\n"
		debugContent := m.modelView.spinners.Debug()
		debugContent += "\nDebug console output:\n" + m.modelView.debugOutput.String()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}
