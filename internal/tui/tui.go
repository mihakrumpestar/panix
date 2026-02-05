package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	workflow          *workflow.Workflow
	quitting          bool
	err               error
	modelView         modelView
	rawKeyReader      *tui_raw_key_reader.RawKeyReader
	notification      string
	notificationColor lipgloss.Style
	notificationTime  time.Time
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

	// Ignore SIGINT so ctrl+c can be handled as a keybinding
	// instead of terminating the process
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		for range sigChan {
			// Ignore SIGINT - it will be handled as a keybinding
		}
	}()

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
			viewports:   tui_viewports.NewViewports(dimensions, workflow.State().Conf.ColorScheme, debugOutput, workflow.State().Conf.Flags.Tui.CommandOutputMaxHeight),
			debugOutput: debugOutput,
		},
		rawKeyReader: rawKeyReader,
	},
		tea.WithInput(rawKeyReader),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	debugOutput.WriteString("TUI initialized\n")

	cpuprofile := workflow.State().Conf.Flags.Logging.Cpuprofile
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
		// Return error from model if present (takes precedence over bubbletea error)
		if finalModel.err != nil {
			return finalModel.err
		}
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
				m.modelView.debugOutput.WriteString("Error: " + err.Error() + "\n")
				// Don't treat context.Canceled as an error (user pressed 'q')
				if err != context.Canceled {
					m.err = err
					msg = errMsg{err}
				}
				return
			}

			m.modelView.debugOutput.WriteString("All ok\n")
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

	// Handle notification timer expiration
	case notificationMsg:
		m.clearNotification()

		// re‐arm for the next keystroke
	case tui_raw_key_reader.RawKeyReaderMsg:
		cmds = append(cmds, m.rawKeyReader.Next())

	case tea.KeyMsg:
		return m.HandleKeyInput(msg)

	case tea.WindowSizeMsg:
		dimensions := m.modelView.dimensions

		// Ensure minimum width
		dimensions.Width = max(msg.Width, 40)
		dimensions.Height = msg.Height

	// Update spinners
	case spinner.TickMsg:
		cmds = append(cmds, m.modelView.spinners.Update(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// Always render main content to update all viewports (even in fullscreen)
	mainContent := m.ViewMainContent()

	if m.quitting {
		// When quitting, return the full content without viewport
		return zone.Scan(mainContent)
	}

	// Check if we're in fullscreen mode
	if m.modelView.viewports.IsFullscreen() {
		fullscreenXpath := m.modelView.viewports.GetFullscreenXpath()

		// Get the updated content from the viewport (now refreshed by ViewMainContent)
		content := m.modelView.viewports.GetViewportContent(fullscreenXpath)
		if content == "" {
			// If no content for fullscreen viewport, exit fullscreen mode
			m.modelView.viewports.ExitFullscreen()
		} else {
			// Render fullscreen viewport with updated content
			fullscreenViewport := m.modelView.viewports.RenderFullscreenViewport(fullscreenXpath, content)

			var builder strings.Builder
			builder.WriteString(fullscreenViewport)

			// Add keybindings at the bottom (footer is still shown)
			builder.WriteString(m.ViewKeybindings(builder))

			return zone.Scan(builder.String())
		}
	}

	// Use the special main viewport method
	mainViewport := m.modelView.viewports.GetOrCreateMainViewport(mainContent)

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
		builder.WriteString(m.workflow.State().Conf.ColorScheme.Error.Color.Render(errorHeader + errorContent))
	}

	if m.workflow.State().Conf.Flags.Logging.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := m.modelView.spinners.Debug()
		debugContent += m.modelView.viewports.Debug()
		debugContent += m.workflow.State().Conf.TargetsLogs.Debug()
		debugContent += "\nDebug console output:\n" + m.modelView.debugOutput.String()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}
