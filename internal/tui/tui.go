package tui

import (
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/pkg/errors"
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
	workflow     *workflow.Workflow
	quitting     bool
	err          error
	modelView    modelView
	rawKeyReader *RawKeyReader
}

type modelView struct {
	mode        TuiViewMode
	dimensions  *Dimensions
	spinners    *Spinners
	viewports   *Viewports
	debugOutput *strings.Builder
	colors      ColorScheme
	//viewport viewport.Model
}

type Dimensions struct {
	width  int
	height int
}

func NewTui(workflow *workflow.Workflow) error {
	zone.NewGlobal()
	defer zone.Close()

	defaultModelView := TuiViewModeAll
	if !slices.Contains(workflow.Phases(), workflow_definition.PhaseStatus) {
		defaultModelView = TuiViewModeLogs
	}

	dimensions := &Dimensions{
		width:  120, // Initial dimensions before tea.WindowSizeMsg
		height: 120, // Initial dimensions before tea.WindowSizeMsg
	}

	debugOutput := &strings.Builder{}

	colors := DefaultColorScheme()

	rawKeyReader := NewRawKeyReader(os.Stdin, 1024)

	//stdin := io.TeeReader(os.Stdin, os.Stdout)

	p := tea.NewProgram(model{
		workflow: workflow,
		modelView: modelView{
			mode:        defaultModelView,
			dimensions:  dimensions,
			spinners:    NewSpinners(),
			viewports:   NewViewports(dimensions, debugOutput, colors),
			debugOutput: debugOutput,
			colors:      colors,
		},
		rawKeyReader: rawKeyReader,
	},
		tea.WithInput(rawKeyReader),
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
		m.rawKeyReader.Next(),
	)
}

func (m model) startWorkflow() tea.Cmd {
	return func() tea.Msg {
		// Use a closure to handle both panic recovery and error handling
		msg := tea.Quit()

		func() {
			defer func() {
				if r := recover(); r != nil {
					var err error
					if e, ok := r.(error); ok {
						err = fmt.Errorf("panic recovered: %w\n\n%s", e, string(debug.Stack()))
					} else {
						err = fmt.Errorf("panic recovered: %v\n\n%s", r, string(debug.Stack()))
					}
					m.err = err
					msg = errMsg{err}
				}
			}()

			err := m.workflow.Start()
			if err != nil {
				m.modelView.debugOutput.WriteString("Error: " + err.Error())
				m.err = err
				msg = errMsg{err}
				return
			}

			m.modelView.debugOutput.WriteString("All ok")
		}()

		return msg
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
	cmds = append(cmds, m.modelView.viewports.Update(msg))

	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err
		return m, tea.Quit

	// Trigger re-render when update is received
	case stateUpdateHookMsg:
		cmds = append(cmds, m.stateUpdateHook())

		// re‐arm for the next keystroke
	case RawKeyReaderMsg:
		cmds = append(cmds, m.rawKeyReader.Next())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			m.workflow.Cancel()()

			// Block quiting until Workflow finalizes and terminates its tasks
			ctx := m.workflow.Ctx()
			<-ctx.Done()

			if m.err != nil && ctx.Err() != nil {
				m.err = errors.Wrap(m.err, ctx.Err().Error())
			} else if ctx.Err() != nil {
				m.err = ctx.Err()
			}

			m.modelView.debugOutput.WriteString("CTX done")

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
		dimensions := m.modelView.dimensions

		dimensions.width = msg.Width
		dimensions.height = msg.Height

	// Update spinners
	case spinner.TickMsg:
		cmds = append(cmds, m.modelView.spinners.Update(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var builder strings.Builder

	colors := m.modelView.colors

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
		errorHeader := "\n\n=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", m.err.Error())
		builder.WriteString(colors.Error.Render(errorHeader + errorContent))
	}

	if m.workflow.State().Conf.Global.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := m.modelView.spinners.Debug()
		debugContent += m.modelView.viewports.Debug()
		debugContent += "\nDebug console output:\n" + m.modelView.debugOutput.String()
		builder.WriteString(debugHeader + debugContent)
	}

	if m.quitting {
		builder.WriteString("\n")
		return builder.String()
	}

	return zone.Scan(builder.String())
}
