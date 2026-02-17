package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_notifications"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_viewports"
	"github.com/mihakrumpestar/panix/internal/workflow"
	zerolog "github.com/rs/zerolog/log"
)

type workflowUpdateHookMsg struct{}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Model holds the complete TUI state.
type model struct {
	ctx          context.Context
	conf         *config.Config
	dimensions   *tui_viewports.Dimensions
	quitting     bool
	resetable    resetable // Has to be able to reset
	notification *tui_notifications.Notification
}

type resetable struct {
	err       error
	workflow  *workflow.Workflow // Has to be able to reset
	spinners  *tui_spinners.Spinners
	viewports *tui_viewports.Viewports
}

// NewTui initializes and runs the TUI application.
func NewTui(ctx context.Context, conf *config.Config) error {
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

	cpuprofile := conf.Flags.Logging.CPUProfile
	if cpuprofile != "" {
		if err := startCPUProfile(cpuprofile); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	p := tea.NewProgram(&model{
		ctx:          ctx,
		conf:         conf,
		dimensions:   dimensions,
		notification: tui_notifications.New(),
	},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	m, err := p.Run()
	if err != nil {
		return err
	}

	finalModel, ok := m.(*model)
	if !ok {
		panic("internal error: type casting for model failed")
	}

	err = finalModel.resetable.err
	if err == nil {
		return err
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

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		m.startWorkflow(),
		m.workflowUpdateHook(),
	)
}

// workflowDoneMsg signals the workflow has completed
type workflowDoneMsg struct{}

// restartMsg signals the workflow should be restarted
type restartMsg struct{}

func (m *model) startWorkflow() tea.Cmd {
	return func() tea.Msg {
		workflow, err := workflow.NewWorkflow(m.ctx, m.conf)
		if err != nil {
			return errMsg{err}
		}

		spinners, err := tui_spinners.NewSpinners()
		if err != nil {
			return errMsg{err}
		}

		m.resetable = resetable{
			workflow:  workflow,
			spinners:  spinners,
			viewports: tui_viewports.NewViewports(m.dimensions, m.conf.ColorScheme, nil, m.conf.Flags.Tui.CommandOutputMaxHeight),
		}

		err = workflow.CreateWorkflow()
		if err != nil {
			zerolog.Error().Err(err).Msg("Workflow execution failed")
			if err != context.Canceled {
				return errMsg{err}
			}
		} else {
			zerolog.Info().Msg("Workflow completed successfully")
		}

		return workflowDoneMsg{}
	}
}

func (m *model) workflowUpdateHook() tea.Cmd {
	return func() tea.Msg {
		if m.resetable.workflow != nil {
			<-m.resetable.workflow.WaitForUpdate()
			zerolog.Debug().Msg("workflowUpdateHook triggered")
		} else {
			time.Sleep(20 * time.Millisecond)
			zerolog.Debug().Msg("workflowUpdateHook was nil")
		}

		return workflowUpdateHookMsg{}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	cmd = tea.Batch(cmd, m.resetable.spinners.ProcessPendingTicks())
	cmd = tea.Batch(cmd, m.resetable.viewports.Update(msg))

	switch msg := msg.(type) {
	case errMsg:
		m.resetable.err = msg.err
		m.quitting = true
		return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)

	case workflowDoneMsg:
		// Only exit automatically if exitOnComplete flag is set
		if m.conf.Flags.ExitOnComplete {
			m.quitting = true
			return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)
		}
		// Stay open - user can press 'q' to quit or 'r' to retry
		return m, nil

	case restartMsg:
		// Cancel current workflow
		m.resetable.workflow.Cancel()

		// Recreate the workflow
		return m, m.startWorkflow()

	case workflowUpdateHookMsg:
		cmd = tea.Batch(cmd, m.workflowUpdateHook())

	case tui_notifications.Msg:
		m.notification.Clear()

	case tea.KeyMsg:
		return m.HandleKeyInput(msg)

	case tea.WindowSizeMsg:
		dimensions := m.dimensions
		dimensions.Width = max(msg.Width, 40)
		dimensions.Height = msg.Height

	case spinner.TickMsg:
		cmd = tea.Batch(cmd, m.resetable.spinners.Update(msg))
	}

	return m, cmd
}

func (m *model) View() string {
	if m.resetable.workflow == nil {
		return ""
	}

	mainContent := m.ViewMainContent()

	if m.quitting {
		return zone.Scan(mainContent)
	}

	if m.resetable.viewports.IsFullscreen() {
		result := m.renderFullscreen()
		if result != "" {
			return result
		}
	}

	footer := m.ViewFooter(strings.Builder{})
	footerHeight := lipgloss.Height(footer)
	mainViewport := m.resetable.viewports.GetOrCreateMainViewport(mainContent, footerHeight)

	var builder strings.Builder
	builder.WriteString(mainViewport)
	builder.WriteString(footer)

	return zone.Scan(builder.String())
}

func (m *model) renderFullscreen() string {
	fullscreenXpath := m.resetable.viewports.GetFullscreenXpath()
	content := m.resetable.viewports.GetViewportContent(fullscreenXpath)

	if content == "" {
		m.resetable.viewports.ExitFullscreen()
		return ""
	}

	footer := m.ViewFooter(strings.Builder{})
	footerHeight := lipgloss.Height(footer)
	fullscreenViewport := m.resetable.viewports.RenderFullscreenViewport(fullscreenXpath, content, footerHeight)

	var builder strings.Builder
	builder.WriteString(fullscreenViewport)
	builder.WriteString(footer)

	return zone.Scan(builder.String())
}

// ViewMainContent generates the main content that is in a viewport
func (m *model) ViewMainContent() string {
	var builder strings.Builder

	builder.WriteString(m.ViewStatsTable())
	builder.WriteString(m.ViewPhaseStatus())
	builder.WriteString(m.ViewBuildLogs())

	if m.resetable.err != nil {
		errorHeader := "=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", m.resetable.err.Error())
		builder.WriteString(m.conf.ColorScheme.Error.Color.Render(errorHeader + errorContent))
	}

	if m.conf.Flags.Logging.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := m.resetable.spinners.Debug()
		debugContent += m.resetable.viewports.Debug()
		debugContent += m.resetable.workflow.State().TargetsLogs.Debug()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}
