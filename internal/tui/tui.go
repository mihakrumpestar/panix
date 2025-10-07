package tui

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/workflow"
)

type stateUpdateHookMsg struct{}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type model struct {
	workflow     *workflow.Workflow
	quitting     bool
	err          error
	modelView    modelView
	rawKeyReader *RawKeyReader
}

type modelView struct {
	dimensions  *Dimensions
	spinners    *Spinners
	viewports   *Viewports
	debugOutput *strings.Builder
}

type Dimensions struct {
	width  int
	height int
}

func NewTui(workflow *workflow.Workflow) error {
	zone.NewGlobal()
	defer zone.Close()

	dimensions := &Dimensions{
		width:  120, // Initial dimensions before tea.WindowSizeMsg
		height: 120, // Initial dimensions before tea.WindowSizeMsg
	}

	debugOutput := &strings.Builder{}

	rawKeyReader := NewRawKeyReader(os.Stdin, 1024)

	//stdin := io.TeeReader(os.Stdin, os.Stdout)

	p := tea.NewProgram(model{
		workflow: workflow,
		modelView: modelView{
			dimensions:  dimensions,
			spinners:    NewSpinners(),
			viewports:   NewViewports(dimensions, workflow.State().Conf.Tui.ColorScheme, debugOutput),
			debugOutput: debugOutput,
		},
		rawKeyReader: rawKeyReader,
	},
		tea.WithInput(rawKeyReader),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	debugOutput.WriteString("TUI initialized")

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

			err := m.workflow.CreateWorkflow()
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
		_, ok := <-m.workflow.WaitForUpdate()
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
		return m.HandleKeyInput(msg)

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

	builder.WriteString(m.ViewStatusTable())
	builder.WriteString(m.ViewPhaseStatus())
	builder.WriteString(m.ViewStateLogs())

	if m.err != nil {
		errorHeader := "\n\n=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", m.err.Error())
		builder.WriteString(m.workflow.State().Conf.Tui.ColorScheme.Error.Render(errorHeader + errorContent))
	}

	if m.workflow.State().Conf.Flags.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := m.modelView.spinners.Debug()
		debugContent += m.modelView.viewports.Debug()
		debugContent += "\nDebug console output:\n" + m.modelView.debugOutput.String()
		builder.WriteString(debugHeader + debugContent)
	}

	if m.quitting {
		return zone.Scan(builder.String())
	}

	builder.WriteString(m.ViewKeybindings(builder))

	return zone.Scan(builder.String())
}
