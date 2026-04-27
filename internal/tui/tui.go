package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/profile"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/tui/buildlogs"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var ErrTypeAssertionFinalModel = errors.New("internal error: type assertion failed for finalModel")

const (
	workflowUpdateHookPollInterval  = 20 * time.Millisecond
	workflowUpdateHookThrottleDelay = 100 * time.Millisecond
)

type (
	workflowUpdateHookMsg struct{}

	// restartMsg signals the workflow should be restarted.
	restartMsg struct{}
)

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
	isSnapshot         bool
	resetable          atomic.Pointer[resetable]
	lastWorkflowUpdate time.Time

	header    *header.Header
	buildLogs *buildlogs.BuildLogs
	footer    *footer.Footer
	spinners  *spinners.Spinners
}

// New initializes and runs the TUI application.
func New(ctx context.Context, conf *config.Config, isSnapshot bool) error {
	zone.NewGlobal()

	defer zone.Close()

	// Handle SIGINT as a keybinding instead of terminating the process
	defer setupSIGINTHandler(ctx)()

	profileStop, err := profile.Start(conf.Flags.Profile)
	if err != nil {
		return errors.Wrap(err, "failed to start profiling")
	}
	defer profileStop()

	mdl := &model{
		ctx:        ctx,
		conf:       conf,
		dimensions: &viewports.Dimensions{},
		isSnapshot: isSnapshot,

		header:   header.New(isSnapshot, conf.Snapshot),
		spinners: spinners.NewSpinners(),
	}

	mdl.footer = footer.New(mdl.keyDefs(), conf)

	program := tea.NewProgram(mdl)

	m, err := program.Run()
	if err != nil {
		return errors.Wrap(err, "TUI runtime error")
	}

	finalModel, ok := m.(*model)
	if !ok {
		return ErrTypeAssertionFinalModel
	}

	if finalModel.quitting {
		content := finalModel.header.View(finalModel.dimensions.Width, finalModel.conf.ColorScheme).Content
		content += finalModel.viewMainContent()
		fmt.Println(content)
	}

	r := finalModel.resetable.Load()
	if r != nil {
		return r.err
	}

	return nil
}

func (m *model) Init() tea.Cmd {
	if m.isSnapshot {
		m.resetable.Store(&resetable{
			viewports: viewports.NewViewports(m.dimensions, m.conf),
		})

		m.conf.Fleet.RecalculateCachesOnly(m.conf.Phases)

		return tea.RequestWindowSize
	}

	return tea.Batch(
		tea.RequestWindowSize,
		m.startResetableWorkflow(),
		m.workflowUpdateHook(),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	resetable := m.resetable.Load()
	if resetable != nil {
		cmd = tea.Batch(cmd, m.spinners.ProcessPendingTicks())
		cmd = tea.Batch(cmd, resetable.viewports.Update(msg))
		cmd = tea.Batch(cmd, m.footer.Update(msg))
		cmd = tea.Batch(cmd, m.spinners.Update(msg))
	}

	switch msg := msg.(type) {
	case errMsg:
		resetable.err = msg.err

		m.conf.Fleet.Recalculate(m.conf.Phases)
		logFinalState(m.conf)

		log.Error().Err(msg.err).Msg("errMsg")

		m.quitting = true

		return m, tea.Batch(cmd, tea.Quit)

	case workflowDoneMsg:
		return m.handleWorkflowDone(cmd)

	case restartMsg:
		cmd = tea.Batch(cmd, m.restartWorkflow())

	case workflowUpdateHookMsg:
		cmd = tea.Batch(cmd, m.workflowUpdateHook())

	case tea.KeyPressMsg:
		_, keyCmd := m.HandleKeyInput(msg)
		cmd = tea.Batch(cmd, keyCmd)

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
	if resetable == nil || m.dimensions.Height == 0 || m.dimensions.Width == 0 {
		return view
	}

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf)
	}

	header := m.header.View(m.dimensions.Width, m.conf.ColorScheme)
	mainContent := m.viewMainContent()
	footer := m.footer.View(m.dimensions.Width, m.conf.ColorScheme)

	headerFooterHeight := header.Height + footer.Height

	var main string
	if resetable.viewports.IsFullscreen() {
		main = m.renderFullscreenViewport(headerFooterHeight)
	} else {
		main = resetable.viewports.GetOrCreateMainViewport(mainContent, headerFooterHeight)
	}

	var builder strings.Builder

	builder.WriteString(header.Content)
	builder.WriteString(main)
	builder.WriteString(footer.Content)

	view.SetContent(zone.Scan(builder.String()))

	return view
}

func (m *model) handleWorkflowDone(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if m.isSnapshot {
		return m, cmd
	}

	log.Debug().Msg("workflowDoneMsg")

	m.conf.Fleet.Recalculate(m.conf.Phases)
	logFinalState(m.conf)

	if m.conf.Flags.Snapshot.OnExit {
		m.captureSnapshot(config.SnaphsotReasonExit)
	}

	if m.conf.Flags.ExitOnComplete {
		m.quitting = true

		return m, tea.Batch(cmd, tea.Quit)
	}

	// Stay open — user can press 'q' to quit or 'r' to retry

	return m, cmd
}

func (m *model) viewMainContent() string {
	resetable := m.resetable.Load()
	if resetable == nil {
		return ""
	}

	if m.isSnapshot {
		m.conf.Fleet.RecalculateCachesOnly(m.conf.Phases)
	} else {
		m.conf.Fleet.Recalculate(m.conf.Phases)
	}

	var builder strings.Builder

	if !m.conf.Flags.DryRun {
		if slices.Contains(m.conf.Phases, phase.Inspect) {
			builder.WriteString(m.conf.Fleet.StatsTable.View(m.dimensions.Width, m.conf.ColorScheme))
		}

		builder.WriteString(m.conf.Fleet.PhaseStatus.View(m.dimensions.Width, m.conf.ColorScheme))
	}

	builder.WriteString(m.buildLogs.View(resetable.viewports, m.spinners))

	if resetable.err != nil {
		errorHeader := "\n\n=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", resetable.err.Error())
		builder.WriteString(m.conf.ColorScheme.Error.Color.Render(errorHeader + errorContent))
	}

	if m.conf.Flags.Logging.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := fmt.Sprintf("terminal - h: %d, w: %d\n", m.dimensions.Height, m.dimensions.Width)
		debugContent += fmt.Sprintf("header - h: %d\n", m.header.View(m.dimensions.Width, m.conf.ColorScheme).Height)
		debugContent += fmt.Sprintf("footer - h: %d\n", m.footer.View(m.dimensions.Width, m.conf.ColorScheme).Height)
		debugContent += m.spinners.Debug()
		debugContent += resetable.viewports.Debug()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}

func (m *model) workflowUpdateHook() tea.Cmd {
	return func() tea.Msg {
		resetable := m.resetable.Load()
		if resetable == nil || resetable.workflow == nil {
			time.Sleep(workflowUpdateHookPollInterval)

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

func (m *model) renderFullscreenViewport(footerHeaderHeight int) string {
	resetable := m.resetable.Load()
	fullscreenXpath := resetable.viewports.GetFullscreenXpath()
	content := resetable.viewports.GetViewportContent(fullscreenXpath)

	if content == "" {
		resetable.viewports.ExitFullscreen()

		return ""
	}

	fullscreenViewport := resetable.viewports.RenderFullscreenViewport(fullscreenXpath, content, footerHeaderHeight)

	return fullscreenViewport
}

func (m *model) handleMouseClick(msg tea.MouseClickMsg) {
	if m.conf.Fleet.StatsTable.HandleMouseClick(msg) {
		m.conf.Fleet.PhaseStatus.Reset()

		return
	}

	if m.conf.Fleet.PhaseStatus.HandleMouseClick(msg) {
		m.conf.Fleet.StatsTable.Reset()
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
