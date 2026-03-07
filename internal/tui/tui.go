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
	"github.com/mihakrumpestar/panix/internal/pkg/tui/notifications"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

const (
	initialWidth  = 80
	initialHeight = 24

	workflowUpdateHookPollInterval  = 20 * time.Millisecond
	workflowUpdateHookThrottleDelay = 100 * time.Millisecond
)

type workflowUpdateHookMsg struct{}

type errMsg struct { //nolint:errname
	err error
}

func (e errMsg) Error() string {
	return e.err.Error()
}

// Model holds the complete TUI state.
type model struct {
	ctx                context.Context
	conf               *config.Config
	dimensions         *viewports.Dimensions
	quitting           bool
	resetable          atomic.Pointer[resetable]
	notification       *notifications.Notification
	lastWorkflowUpdate time.Time
}

// NewTui initializes and runs the TUI application.
func NewTui(ctx context.Context, conf *config.Config) error {
	zone.NewGlobal()
	defer zone.Close()

	// Handle SIGINT as a keybinding instead of terminating the process
	defer setupSIGINTHandler(ctx)()

	dimensions := &viewports.Dimensions{
		Width:  initialWidth,
		Height: initialHeight,
	}

	cpuProfile := conf.Flags.Logging.CPUProfile
	if cpuProfile != "" {
		stopCPUProfile, err := startCPUProfile(cpuProfile)
		if err != nil {
			return err
		}
		defer stopCPUProfile()
	}

	program := tea.NewProgram(&model{
		ctx:          ctx,
		conf:         conf,
		dimensions:   dimensions,
		notification: notifications.New(),
	})

	m, err := program.Run()
	if err != nil {
		return errors.Wrap(err, "TUI runtime error")
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
	file, err := os.Create(path) // #nosec G304 -- path comes from controlled configuration flag
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CPU profile file")
	}

	err = pprof.StartCPUProfile(file)
	if err != nil {
		err := file.Close()
		if err != nil {
			log.Error().Err(err).Msg("failed to close CPU profile file")
		}

		return nil, errors.Wrap(err, "failed to start CPU profile")
	}

	return func() {
		pprof.StopCPUProfile()

		err := file.Close()
		if err != nil {
			log.Error().Err(err).Msg("failed to close CPU profile file")
		}
	}, nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestWindowSize,
		m.startWorkflow(),
		m.workflowUpdateHook(),
	)
}

// restartMsg signals the workflow should be restarted.
type restartMsg struct{}

func (m *model) workflowUpdateHook() tea.Cmd {
	return func() tea.Msg {
		resetable := m.resetable.Load()
		if resetable == nil || resetable.workflow == nil {
			time.Sleep(workflowUpdateHookPollInterval)
			log.Debug().Msg("workflowUpdateHook was nil")

			return workflowUpdateHookMsg{}
		}

		<-resetable.workflow.WaitForUpdate()

		now := time.Now()
		elapsed := now.Sub(m.lastWorkflowUpdate)

		if elapsed < workflowUpdateHookThrottleDelay {
			time.Sleep(workflowUpdateHookThrottleDelay - elapsed)
		}

		m.lastWorkflowUpdate = time.Now()

		return workflowUpdateHookMsg{}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	resetable := m.resetable.Load()
	if resetable != nil {
		cmd = tea.Batch(cmd, resetable.spinners.ProcessPendingTicks())
		cmd = tea.Batch(cmd, resetable.viewports.Update(msg))
		cmd = tea.Batch(cmd, m.notification.Update(msg))
		cmd = tea.Batch(cmd, resetable.spinners.Update(msg))
	}

	switch msg := msg.(type) {
	case errMsg:
		resetable.err = msg.err
		m.quitting = true

		return m, tea.Quit

	case workflowDoneMsg:
		log.Debug().Msg("workflowDoneMsg")

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
		m.dimensions.Width = msg.Width
		m.dimensions.Height = msg.Height
	}

	return m, cmd
}

func (m *model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion

	resetable := m.resetable.Load()
	if resetable == nil || resetable.workflow == nil {
		return view
	}

	mainContent := m.ViewMainContent()

	if m.quitting {
		return view
	}

	if resetable.viewports.IsFullscreen() {
		result := m.renderFullscreen()
		if result != "" {
			view.SetContent(result)

			return view
		}
	}

	footer := m.ViewFooter()
	footerHeight := lipgloss.Height(footer)
	mainViewport := resetable.viewports.GetOrCreateMainViewport(mainContent, footerHeight)

	var builder strings.Builder

	builder.WriteString(mainViewport)
	builder.WriteString(footer)

	view.SetContent(zone.Scan(builder.String()))

	return view
}

func (m *model) renderFullscreen() string {
	resetable := m.resetable.Load()
	fullscreenXpath := resetable.viewports.GetFullscreenXpath()
	content := resetable.viewports.GetViewportContent(fullscreenXpath)

	if content == "" {
		resetable.viewports.ExitFullscreen()

		return ""
	}

	footer := m.ViewFooter()
	footerHeight := lipgloss.Height(footer)
	fullscreenViewport := resetable.viewports.RenderFullscreenViewport(fullscreenXpath, content, footerHeight)

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

	resetable := m.resetable.Load()
	if resetable.err != nil {
		errorHeader := "\n\n=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", resetable.err.Error())
		builder.WriteString(m.conf.ColorScheme.Error.Color.Render(errorHeader + errorContent))
	}

	if m.conf.Flags.Logging.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := resetable.spinners.Debug()
		debugContent += resetable.viewports.Debug()
		debugContent += resetable.workflow.State().TargetsLogs.Debug()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}

func (m *model) handleMouseClick(msg tea.MouseClickMsg) {
	resetable := m.resetable.Load()
	if resetable.statsTable.HandleMouseClick(msg) {
		resetable.phaseStatus.Reset()

		return
	}

	if resetable.phaseStatus.HandleMouseClick(msg) {
		resetable.statsTable.Reset()
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
			case <-sigChan: // SIGINT is handled as a keybinding
			}
		}
	}()

	return func() {
		signal.Stop(sigChan)
	}
}
