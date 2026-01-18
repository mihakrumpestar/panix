package tui

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_raw_key_reader"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_viewports"
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
	rawKeyReader *tui_raw_key_reader.RawKeyReader
}

type modelView struct {
	dimensions  *tui_viewports.Dimensions
	spinners    *tui_spinners.Spinners
	viewports   *tui_viewports.Viewports
	debugOutput *strings.Builder
}

func NewTui(workflow *workflow.Workflow) error {
	zone.NewGlobal()
	defer zone.Close()

	dimensions := &tui_viewports.Dimensions{
		Width:  120, // Initial dimensions before tea.WindowSizeMsg
		Height: 120, // Initial dimensions before tea.WindowSizeMsg
	}

	debugOutput := &strings.Builder{}

	rawKeyReader := tui_raw_key_reader.NewRawKeyReader(os.Stdin, 1024)

	//stdin := io.TeeReader(os.Stdin, os.Stdout)

	p := tea.NewProgram(model{
		workflow: workflow,
		modelView: modelView{
			dimensions:  dimensions,
			spinners:    tui_spinners.NewSpinners(),
			viewports:   tui_viewports.NewViewports(dimensions, workflow.State().Conf.Tui.ColorScheme, debugOutput),
			debugOutput: debugOutput,
		},
		rawKeyReader: rawKeyReader,
	},
		tea.WithInput(rawKeyReader),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	debugOutput.WriteString("TUI initialized")

	cpuprofile := workflow.State().Conf.Flags.Cpuprofile
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	m, err := p.Run() // Blocking

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
	case tui_raw_key_reader.RawKeyReaderMsg:
		cmds = append(cmds, m.rawKeyReader.Next())

	case tea.KeyMsg:
		return m.HandleKeyInput(msg)

	case tea.WindowSizeMsg:
		dimensions := m.modelView.dimensions

		dimensions.Width = msg.Width
		if dimensions.Width < 40 { // Ensure minimum width
			dimensions.Width = 40
		}

		dimensions.Height = msg.Height

	// Update spinners
	case spinner.TickMsg:
		cmds = append(cmds, m.modelView.spinners.Update(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	mainContent := m.ViewMainContent()

	if m.quitting {
		// When quitting, return the full content without viewport
		return zone.Scan(mainContent)
	}

	// Use the special main viewport method
	mainViewport := m.modelView.viewports.GetOrCreateMainViewport(mainContent)
	//mainViewport := mainContent

	var builder strings.Builder
	builder.WriteString(mainViewport)

	// Add keybindings at the bottom
	builder.WriteString(m.ViewKeybindings(builder))

	return zone.Scan(builder.String())
}

// Helpers

// ViewMainContent generates the main content that is in a viewport
func (m model) ViewMainContent() string {
	var builder strings.Builder

	builder.WriteString(m.ViewStatsTable())
	builder.WriteString(m.ViewPhaseStatus())
	builder.WriteString(m.ViewBuildLogs())

	if m.err != nil {
		errorHeader := "=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", m.err.Error())
		builder.WriteString(m.workflow.State().Conf.Tui.ColorScheme.Error.Color.Render(errorHeader + errorContent))
	}

	if m.workflow.State().Conf.Flags.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := m.modelView.spinners.Debug()
		debugContent += m.modelView.viewports.Debug()
		debugContent += "\nDebug console output:\n" + m.modelView.debugOutput.String()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}
