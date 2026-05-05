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

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/tui/buildlogs"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/profile"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
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

	restartMsg struct{}
)

type errMsg struct { //nolint:errname
	err error
}

func (e errMsg) Error() string {
	return e.err.Error()
}

type model struct {
	ctx                context.Context
	conf               *config.Config
	dimensions         *viewports.Dimensions
	quitting           bool
	isSnapshot         bool
	resetable          atomic.Pointer[resetable]
	lastWorkflowUpdate time.Time
	err                error
	contentVersion     uint64
	header             *header.Header
	buildLogs          *buildlogs.BuildLogs
	footer             *footer.Footer
	spinners           *spinners.Spinners
	viewports          *viewports.Viewports
	statsTable         *statstable.StatsTable
	phaseFlow          *phaseflow.PhaseFlow
}

func New(ctx context.Context, conf *config.Config, isSnapshot bool) error {
	defer setupSIGINTHandler(ctx)()

	profileStop, err := profile.Start(conf.Flags.Profile)
	if err != nil {
		return errors.Wrap(err, "failed to start profiling")
	}
	defer profileStop()

	dimensions := &viewports.Dimensions{}

	mdl := &model{
		ctx:        ctx,
		conf:       conf,
		dimensions: dimensions,
		isSnapshot: isSnapshot,

		header:     header.New(isSnapshot, conf.Snapshot, conf.ColorScheme),
		spinners:   spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval),
		viewports:  viewports.New(dimensions, conf.Flags.CommandOutputMaxHeight, conf.ColorScheme.Table.Border, conf.ColorScheme.Table.SelectionHighlightBackground, conf.ColorScheme.Table.SelectionHighlightBorder),
		statsTable: statstable.New(conf.Fleet, conf.ColorScheme),
		phaseFlow:  phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases),
	}

	mdl.footer = footer.New(mdl.keyDefs(), conf, conf.ColorScheme)

	program := zeroterm.NewProgram(mdl /*, render.WithRaw() */)

	err = program.Run()
	if err != nil {
		return errors.Wrap(err, "TUI runtime error")
	}

	if mdl.quitting {
		content := mdl.header.View(mdl.dimensions.Width)

		r := mdl.resetable.Load()
		if r != nil {
			content += mdl.viewMainContent()
		}

		if mdl.err != nil {
			content += fmt.Sprintf("\n\n=== Error ===\n\n%s\n", mdl.err.Error())
		}

		fmt.Println(content)
	}

	if mdl.err != nil {
		return mdl.err
	}

	return nil
}

func (m *model) Init() []zeroterm.Cmd {
	if m.isSnapshot {
		m.conf.Fleet.RecalculateCachesOnly(m.conf.Phases)

		return nil
	}

	return []zeroterm.Cmd{
		m.startResetableWorkflow(),
		m.workflowUpdateHook(),
	}
}

func (m *model) Update(msg zeroterm.Msg) []zeroterm.Cmd {
	var cmds []zeroterm.Cmd

	resetable := m.resetable.Load()
	if resetable != nil {
		cmds = append(cmds, m.viewports.Update(msg))
		cmds = append(cmds, m.footer.Update(msg))
		cmds = append(cmds, m.spinners.Update(msg))
	}

	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err

		m.conf.Fleet.Recalculate(m.conf.Phases)
		logFinalState(m.conf)

		log.Error().Err(msg.err).Msg("errMsg")

		m.quitting = true

		cmds = append(cmds, zeroterm.QuitCmd)

	case workflowDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			log.Error().Err(msg.err).Msg("workflowDoneMsg error")
		}

		return m.handleWorkflowDone(cmds)

	case restartMsg:
		cmds = append(cmds, m.restartWorkflow())

	case workflowUpdateHookMsg:
		cmds = append(cmds, m.workflowUpdateHook())

	case zeroterm.PostRenderMsg:
		cmds = append(cmds, m.spinners.ProcessPendingTicks())

	case zeroterm.KeyPressMsg:
		keyCmds := m.HandleKeyInput(msg)
		cmds = append(cmds, keyCmds...)

	case zeroterm.MouseClickMsg:
		m.handleMouseClick(msg)

	case zeroterm.WindowSizeMsg:
		m.dimensions.Width = msg.Width
		m.dimensions.Height = msg.Height
	}

	return cmds
}

func (m *model) Render() []string {
	resetable := m.resetable.Load()
	if resetable == nil || m.dimensions.Height == 0 || m.dimensions.Width == 0 {
		return nil
	}

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf, m.statsTable, m.phaseFlow)
	}

	header := m.header.View(m.dimensions.Width)
	mainContent := m.viewMainContent()
	footer := m.footer.View(m.dimensions.Width, m.conf.ColorScheme)

	headerFooterHeight := style.CountLines(header) - 1 + style.CountLines(footer)

	m.contentVersion++

	var main string
	if m.viewports.IsFullscreen() {
		main = m.renderFullscreenViewport(headerFooterHeight)
	} else {
		main = m.viewports.GetOrCreateMainViewport(mainContent, m.contentVersion, headerFooterHeight)
	}

	var builder strings.Builder

	builder.WriteString(header)
	builder.WriteString(main)
	builder.WriteString(footer)

	renderStr := builder.String()

	return strings.Split(renderStr, "\n")
}

func (m *model) handleWorkflowDone(cmds []zeroterm.Cmd) []zeroterm.Cmd {
	if m.isSnapshot {
		return cmds
	}

	log.Debug().Msg("workflowDoneMsg")

	m.conf.Fleet.Recalculate(m.conf.Phases)
	logFinalState(m.conf)

	if m.conf.Flags.Snapshot.OnExit {
		m.captureSnapshot(config.SnaphsotReasonExit)
	}

	if m.conf.Flags.ExitOnComplete {
		m.quitting = true

		return append(cmds, zeroterm.QuitCmd)
	}

	return cmds
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

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf, m.statsTable, m.phaseFlow)
	}

	var builder strings.Builder

	contentWidth := m.viewports.ContentWidth()

	if !m.conf.Flags.DryRun {
		if slices.Contains(m.conf.Phases, phase.Inspect) {
			builder.WriteString(m.statsTable.View(contentWidth))
		}

		builder.WriteString(m.phaseFlow.View(contentWidth))
	}

	builder.WriteString(m.buildLogs.View(m.viewports, m.spinners))

	if m.err != nil {
		errorHeader := "\n\n=== Error ===\n"
		errorContent := fmt.Sprintf("\n%s\n", m.err.Error())
		builder.WriteString(m.conf.ColorScheme.Error.Color.Render(errorHeader + errorContent))
	}

	if m.conf.Flags.Logging.Debug {
		debugHeader := "\n\n=== Debug ===\n"
		debugContent := fmt.Sprintf("terminal - h: %d, w: %d\n", m.dimensions.Height, m.dimensions.Width)
		debugContent += fmt.Sprintf("header - h: %d\n", style.CountLines(m.header.View(m.dimensions.Width))-1)
		debugContent += fmt.Sprintf("footer - h: %d\n", style.CountLines(m.footer.View(m.dimensions.Width, m.conf.ColorScheme)))
		debugContent += m.spinners.Debug()
		debugContent += m.viewports.Debug()
		builder.WriteString(debugHeader + debugContent)
	}

	return builder.String()
}

func (m *model) workflowUpdateHook() zeroterm.Cmd {
	return func() zeroterm.Msg {
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
	if resetable == nil {
		return ""
	}

	fullscreenXpath := m.viewports.GetFullscreenXpath()
	content := m.viewports.GetViewportContent(fullscreenXpath)

	if content == "" {
		m.viewports.ExitFullscreen()

		return ""
	}

	fullscreenViewport := m.viewports.RenderFullscreenViewport(fullscreenXpath, content, m.contentVersion, footerHeaderHeight)

	return fullscreenViewport
}

func (m *model) handleMouseClick(msg zeroterm.MouseClickMsg) {
	if m.statsTable.HandleMouseClick(msg) {
		m.phaseFlow.Reset()
	}

	if m.phaseFlow.HandleMouseClick(msg) {
		m.statsTable.Reset()
	}
}

func setupSIGINTHandler(ctx context.Context) func() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigChan:
			}
		}
	}()

	return func() {
		signal.Stop(sigChan)
	}
}
