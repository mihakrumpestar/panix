package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/tui/buildlogs"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
	"github.com/mihakrumpestar/panix/pkg/profile"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/pkg/errors"
)

var ErrTypeAssertionFinalModel = errors.New("internal error: type assertion failed for finalModel")

type model struct {
	ctx                context.Context
	conf               *config.Config
	dimensions         *viewports.Dimensions
	quitting           bool
	isSnapshot         bool
	workflow           *workflow.Workflow
	lastWorkflowUpdate time.Time
	err                error
	contentVersion     uint64

	header     *header.Header
	buildLogs  *buildlogs.BuildLogs
	footer     *footer.Footer
	spinners   *spinners.Spinners
	viewports  *viewports.Viewports
	statsTable *statstable.StatsTable
	phaseFlow  *phaseflow.PhaseFlow
}

func New(ctx context.Context, conf *config.Config, isSnapshot bool) error {
	defer setupSIGINTHandler(ctx)()

	profileStop, err := profile.Start(conf.Flags.Profile)
	if err != nil {
		return errors.Wrap(err, "failed to start profiling")
	}
	defer profileStop()

	dimensions := &viewports.Dimensions{}

	tableS := conf.ColorScheme.Table

	mdl := &model{
		ctx:        ctx,
		conf:       conf,
		dimensions: dimensions,
		isSnapshot: isSnapshot,

		header:   header.New(isSnapshot, conf.Snapshot, conf.ColorScheme),
		spinners: spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval),
		viewports: viewports.New(dimensions, conf.Flags.CommandOutputMaxHeight, tableS.Border,
			tableS.SelectionHighlightBackground, tableS.SelectionHighlightBorder,
		),
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

		if mdl.workflow != nil {
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
		m.workflowStartCmd(),
		m.workflowUpdateHookCmd(),
	}
}

//nolint:cyclop
func (m *model) Update(msg zeroterm.Msg) zeroterm.Cmd {
	var cmd []zeroterm.Cmd

	if m.workflow != nil {
		cmd = append(cmd, m.viewports.Update(msg))
		cmd = append(cmd, m.footer.Update(msg))
		cmd = append(cmd, m.spinners.Update(msg))
	}

	switch msg := msg.(type) {
	case errMsg:
		cmd = append(cmd, m.handleErrMsgCmd(msg))

		return zeroterm.BatchCmd(cmd...)
	case workflowDoneMsg:
		cmd = append(cmd, m.workflowDoneMsgCmd(msg))

		return zeroterm.BatchCmd(cmd...)
	case restartMsg:
		cmd = append(cmd, m.workflowRestartCmd())
	case retryMsg:
		m.workflowRetry()
	case workflowUpdateHookMsg:
		cmd = append(cmd, m.workflowUpdateHookCmd())
	case zeroterm.PostRenderMsg:
		cmd = append(cmd, m.spinners.ProcessPendingTicks())
	case zeroterm.KeyPressMsg:
		cmd = append(cmd, m.HandleKeyInput(msg))
	case zeroterm.MouseClickMsg:
		m.handleMouseClick(msg)
	case zeroterm.WindowSizeMsg:
		m.dimensions.Width = msg.Width
		m.dimensions.Height = msg.Height
	}

	return zeroterm.BatchCmd(cmd...)
}

func (m *model) Render(buf *linesbuffer.LinesBuffer) {
	if m.workflow == nil || m.dimensions.Height == 0 || m.dimensions.Width == 0 {
		return
	}

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf, m.statsTable, m.phaseFlow)
	}

	header := m.header.View(m.dimensions.Width)
	mainContent := m.viewMainContent()

	var footer string
	if !m.quitting {
		footer = m.footer.View(m.dimensions.Width, m.conf.ColorScheme)
	}

	headerFooterHeight := style.CountLines(header) - 1 + style.CountLines(footer)

	m.contentVersion++

	var main string
	if m.viewports.IsFullscreen() {
		main = m.renderFullscreenViewport(headerFooterHeight)
	} else {
		main = m.viewports.GetOrCreateMainViewport(mainContent, m.contentVersion, headerFooterHeight)
	}

	buf.WriteString(header)
	buf.WriteString(main)

	if !m.quitting {
		buf.WriteString(footer)
	}
}

func (m *model) viewMainContent() string {
	if m.workflow == nil {
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

func (m *model) renderFullscreenViewport(footerHeaderHeight int) string {
	if m.workflow == nil {
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
