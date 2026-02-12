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
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_viewports"
	"github.com/mihakrumpestar/panix/internal/workflow"
)

type stateUpdateHookMsg struct{}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Model holds the complete TUI state.
type model struct {
	workflow          *workflow.Workflow
	quitting          bool
	err               error
	modelView         modelView
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

// NewTui initializes and runs the TUI application.
func NewTui(workflow *workflow.Workflow) error {
	zone.NewGlobal()
	defer zone.Close()

	// Handle SIGINT as a keybinding instead of terminating the process
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	defer signal.Stop(sigChan)

	go func() {
		for range sigChan {
			// SIGINT is handled as a keybinding
		}
	}()

	dimensions := &tui_viewports.Dimensions{
		Width:  80,
		Height: 24,
	}

	debugOutput := &strings.Builder{}
	state := workflow.State()

	p := tea.NewProgram(model{
		workflow: workflow,
		modelView: modelView{
			dimensions:  dimensions,
			spinners:    tui_spinners.NewSpinners(),
			viewports:   tui_viewports.NewViewports(dimensions, state.Conf.ColorScheme, debugOutput, state.Conf.Flags.Tui.CommandOutputMaxHeight),
			debugOutput: debugOutput,
		},
	},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	debugOutput.WriteString("TUI initialized\n")

	cpuprofile := state.Conf.Flags.Logging.CPUProfile
	if cpuprofile != "" {
		if err := startCPUProfile(cpuprofile); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	m, err := p.Run()

	if err != nil {
		return err
	}

	finalModel, ok := m.(model)
	if ok && finalModel.err != nil {
		return finalModel.err
	}

	return nil
}

func startCPUProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return pprof.StartCPUProfile(f)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		m.stateUpdateHook(),
		startWorkflow(m),
	)
}

// workflowDoneMsg signals the workflow has completed
type workflowDoneMsg struct{}

func startWorkflow(m model) tea.Cmd {
	return func() tea.Msg {
		var msg tea.Msg = workflowDoneMsg{}

		defer func() {
			if r := recover(); r != nil {
				m.err = recoverPanic(r)
				msg = errMsg{m.err}
			}
		}()

		if err := m.workflow.CreateWorkflow(); err != nil {
			m.modelView.debugOutput.WriteString("Error: " + err.Error() + "\n")
			if err != context.Canceled {
				m.err = err
				msg = errMsg{err}
			}
		} else {
			m.modelView.debugOutput.WriteString("All ok\n")
		}

		return msg
	}
}

func recoverPanic(r any) error {
	stack := string(debug.Stack())
	if e, ok := r.(error); ok {
		return fmt.Errorf("panic recovered: %w\n\n%s", e, stack)
	}
	return fmt.Errorf("panic recovered: %v\n\n%s", r, stack)
}

func (m model) stateUpdateHook() tea.Cmd {
	return func() tea.Msg {
		_, ok := <-m.workflow.WaitForUpdate()
		if !ok {
			return workflowDoneMsg{}
		}
		return stateUpdateHookMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 8)

	cmds = append(cmds, m.modelView.spinners.SendInitTickIfNotAlready())
	cmds = append(cmds, m.modelView.viewports.Update(msg))

	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)

	case workflowDoneMsg:
		m.quitting = true
		return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)

	case stateUpdateHookMsg:
		cmds = append(cmds, m.stateUpdateHook())

	case notificationMsg:
		m.clearNotification()

	case tea.KeyMsg:
		return m.HandleKeyInput(msg)

	case tea.WindowSizeMsg:
		dimensions := m.modelView.dimensions
		dimensions.Width = max(msg.Width, 40)
		dimensions.Height = msg.Height

	case spinner.TickMsg:
		cmds = append(cmds, m.modelView.spinners.Update(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	mainContent := m.ViewMainContent()

	if m.quitting {
		return zone.Scan(mainContent)
	}

	if m.modelView.viewports.IsFullscreen() {
		result := m.renderFullscreen(mainContent)
		if result != "" {
			return result
		}
	}

	mainViewport := m.modelView.viewports.GetOrCreateMainViewport(mainContent)

	var builder strings.Builder
	builder.WriteString(mainViewport)
	builder.WriteString(m.ViewKeybindings(builder))

	return zone.Scan(builder.String())
}

func (m model) renderFullscreen(mainContent string) string {
	fullscreenXpath := m.modelView.viewports.GetFullscreenXpath()
	content := m.modelView.viewports.GetViewportContent(fullscreenXpath)

	if content == "" {
		m.modelView.viewports.ExitFullscreen()
		return ""
	}

	fullscreenViewport := m.modelView.viewports.RenderFullscreenViewport(fullscreenXpath, content)

	var builder strings.Builder
	builder.WriteString(fullscreenViewport)
	builder.WriteString(m.ViewKeybindings(builder))

	return zone.Scan(builder.String())
}

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
