package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_notifications"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tui_viewports"
	zerolog "github.com/rs/zerolog/log"
)

type workflowUpdateHookMsg struct{}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Model holds the complete TUI state.
type model struct {
	ctx                context.Context
	conf               *config.Config
	dimensions         *tui_viewports.Dimensions
	quitting           bool
	resetable          atomic.Pointer[resetable]
	notification       *tui_notifications.Notification
	lastWorkflowUpdate time.Time
}

// NewTui initializes and runs the TUI application.
func NewTui(ctx context.Context, conf *config.Config) error {
	zone.NewGlobal()
	defer zone.Close()

	// Handle SIGINT as a keybinding instead of terminating the process
	defer setupSIGINTHandler(ctx)()

	dimensions := &tui_viewports.Dimensions{
		Width:  80,
		Height: 24,
	}

	cpuProfile := conf.Flags.Logging.CPUProfile
	if cpuProfile != "" {
		stopCPUProfile, err := startCPUProfile(cpuProfile)
		if err != nil {
			return err
		}
		defer stopCPUProfile()
	}

	p := tea.NewProgram(&model{
		ctx:          ctx,
		conf:         conf,
		dimensions:   dimensions,
		notification: tui_notifications.New(),
	})

	m, err := p.Run()
	if err != nil {
		return err
	}

	finalModel, ok := m.(*model)
	if !ok {
		return fmt.Errorf("internal error: type assertion failed for finalModel")
	}

	if finalModel.quitting {
		content := finalModel.ViewMainContent()
		fmt.Println(content)
	}
	if r := finalModel.resetable.Load(); r != nil {
		return r.err
	}
	return nil
}

func startCPUProfile(path string) (func(), error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	err = pprof.StartCPUProfile(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	return func() {
		pprof.StopCPUProfile()
		_ = f.Close()
	}, nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return tea.RequestWindowSize() },
		m.startWorkflow(),
		m.workflowUpdateHook(),
	)
}

// restartMsg signals the workflow should be restarted
type restartMsg struct{}

func (m *model) workflowUpdateHook() tea.Cmd {
	return func() tea.Msg {
		r := m.resetable.Load()
		if r == nil || r.workflow == nil {
			time.Sleep(20 * time.Millisecond)
			zerolog.Debug().Msg("workflowUpdateHook was nil")
			return workflowUpdateHookMsg{}
		}

		<-r.workflow.WaitForUpdate()

		now := time.Now()
		elapsed := now.Sub(m.lastWorkflowUpdate)
		if elapsed < 100*time.Millisecond {
			time.Sleep(100*time.Millisecond - elapsed)
		}
		m.lastWorkflowUpdate = time.Now()

		return workflowUpdateHookMsg{}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	r := m.resetable.Load()
	if r != nil {
		cmd = tea.Batch(cmd, r.spinners.ProcessPendingTicks())
		cmd = tea.Batch(cmd, r.viewports.Update(msg))
		cmd = tea.Batch(cmd, m.notification.Update(msg))
		cmd = tea.Batch(cmd, r.spinners.Update(msg))
	}

	switch msg := msg.(type) {
	case errMsg:
		r.err = msg.err
		m.quitting = true
		return m, tea.Quit

	case workflowDoneMsg:
		zerolog.Debug().Msg("workflowDoneMsg")

		// Only exit automatically if exitOnComplete flag is set
		if m.conf.Flags.ExitOnComplete {
			m.quitting = true
			return m, tea.Quit
		}
		// Stay open — user can press 'q' to quit or 'r' to retry
		return m, nil

	case restartMsg:
		return m, m.restartWorkflow()

	case workflowUpdateHookMsg:
		cmd = tea.Batch(cmd, m.workflowUpdateHook())

	case tea.KeyPressMsg:
		return m.HandleKeyInput(msg)

	case tea.MouseClickMsg:
		m.handleMouseClick(msg)

	case tea.WindowSizeMsg:
		m.dimensions.Width = max(msg.Width, 40)
		m.dimensions.Height = msg.Height
	}

	return m, cmd
}

func (m *model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	r := m.resetable.Load()
	if r == nil || r.workflow == nil {
		return v
	}

	mainContent := m.ViewMainContent()

	if m.quitting {
		return v
	}

	if r.viewports.IsFullscreen() {
		result := m.renderFullscreen()
		if result != "" {
			v.SetContent(result)
			return v
		}
	}

	footer := m.ViewFooter()
	footerHeight := lipgloss.Height(footer)
	mainViewport := r.viewports.GetOrCreateMainViewport(mainContent, footerHeight)

	var builder strings.Builder
	builder.WriteString(mainViewport)
	builder.WriteString(footer)

	v.SetContent(zone.Scan(builder.String()))
	return v
}

func (m *model) renderFullscreen() string {
	r := m.resetable.Load()
	fullscreenXpath := r.viewports.GetFullscreenXpath()
	content := r.viewports.GetViewportContent(fullscreenXpath)

	if content == "" {
		r.viewports.ExitFullscreen()
		return ""
	}

	footer := m.ViewFooter()
	footerHeight := lipgloss.Height(footer)
	fullscreenViewport := r.viewports.RenderFullscreenViewport(fullscreenXpath, content, footerHeight)

	var builder strings.Builder
	builder.WriteString(fullscreenViewport)
	builder.WriteString(footer)

	return zone.Scan(builder.String())
}

func (m *model) ViewMainContent() string {
	var builder strings.Builder

	builder.WriteString(m.ViewStatsTable())
	builder.WriteString(m.ViewPhaseStatus())
	builder.WriteString(m.ViewBuildLogs())

	r := m.resetable.Load()
	if r.err != nil {
		errorHeader := "\n\n=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", r.err.Error())
		builder.WriteString(m.conf.ColorScheme.Error.Color.Render(errorHeader + errorContent))
	}

	if m.conf.Flags.Logging.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := r.spinners.Debug()
		debugContent += r.viewports.Debug()
		debugContent += r.workflow.State().TargetsLogs.Debug()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}

func (m *model) handleMouseClick(msg tea.MouseClickMsg) {
	r := m.resetable.Load()
	if r.statsTable.HandleMouseClick(msg) {
		r.phaseStatus.Reset()
		return
	}

	if r.phaseStatus.HandleMouseClick(msg) {
		r.statsTable.Reset()
	}
}

// Helpers

// setupSIGINTHandler captures SIGINT signals and routes them as keybindings
// instead of terminating the process. Returns a cleanup function.
func setupSIGINTHandler(ctx context.Context) func() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigChan:
				// SIGINT is handled as a keybinding
			}
		}
	}()

	return func() {
		signal.Stop(sigChan)
	}
}
