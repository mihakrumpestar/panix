package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
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
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/profile"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
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

	mdl.footer = footer.New(mdl.keyDefs(), conf.ColorScheme)

	program := zeroterm.NewProgram(mdl /*, render.WithRaw() */)

	err = program.Run()
	if err != nil {
		return errors.Wrap(err, "TUI runtime error")
	}

	if mdl.quitting {
		buf := buffer.NewLinesBufDiff()

		buf.AppendFrom(mdl.header.Render(mdl.dimensions.Width))

		if mdl.workflow != nil {
			mdl.viewMainContentInto(buf, 0, mdl.header.Len()-1)
		}

		fmt.Println(buf.String())
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

func (m *model) Render(buf *buffer.LinesBufDiff, renderCounter uint64) {
	if m.workflow == nil || m.dimensions.Height == 0 || m.dimensions.Width == 0 {
		return
	}

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf, m.statsTable, m.phaseFlow)
	}

	header := m.header.Render(m.dimensions.Width)
	buf.AppendFrom(header)

	footer := m.footer.Render(m.quitting, m.dimensions.Width)

	headerFooterHeight := header.Len() + footer.Len()

	if m.viewports.IsFullscreen() {
		m.renderFullscreenViewportInto(buf, renderCounter, headerFooterHeight)
	} else {
		m.viewMainContentInto(buf, renderCounter, headerFooterHeight)
	}

	buf.AppendFrom(footer)
}

// viewMainContentInto renders the main content area into buf.
func (m *model) viewMainContentInto(buf *buffer.LinesBufDiff, renderCounter uint64, headerFooterHeight int) {
	if m.workflow == nil {
		return
	}

	if m.isSnapshot {
		m.conf.Fleet.RecalculateCachesOnly(m.conf.Phases)
	} else {
		m.conf.Fleet.Recalculate(m.conf.Phases)
	}

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf, m.statsTable, m.phaseFlow)
	}

	contentWidth := m.viewports.ContentWidth()

	content := buffer.NewLinesBuf()

	if !m.conf.Flags.DryRun {
		if slices.Contains(m.conf.Phases, phase.Inspect) {
			content.AppendFrom(m.statsTable.Render(contentWidth))
		}

		content.AppendFrom(m.phaseFlow.Render(contentWidth))
	}

	content.AppendFrom(m.buildLogs.Render(m.viewports, m.spinners))

	if m.err != nil {
		errContent := [][]byte{
			[]byte{},
			[]byte("=== Error ==="),
			[]byte{},
			[]byte(m.err.Error()),
		}

		m.conf.ColorScheme.Error.Color.RenderInto(content, errContent)
	}

	if m.conf.Flags.Logging.Debug {
		debugContent := [][]byte{
			[]byte{},
			[]byte("=== Debug ==="),
			[]byte{},
			fmt.Appendf(nil, "terminal - h: %d, w: %d\n", m.dimensions.Height, m.dimensions.Width),
			fmt.Appendf(nil, "header - h: %d", m.header.Len()),
			fmt.Appendf(nil, "footer - h: %d\n", m.footer.Len()),
			[]byte{},
		}

		content.WriteLines(debugContent)
	}

	viewport := m.viewports.RenderMainViewport(content, renderCounter, headerFooterHeight)
	buf.AppendFrom(viewport)
}

// renderFullscreenViewportInto renders the fullscreen viewport into buf.
func (m *model) renderFullscreenViewportInto(buf *buffer.LinesBufDiff, renderCounter uint64, footerHeaderHeight int) {
	if m.workflow == nil {
		return
	}

	fullscreenXpath := m.viewports.GetFullscreenXpath()
	content := m.viewports.GetViewportContent(fullscreenXpath)

	if len(content) == 0 {
		m.viewports.ExitFullscreen()

		return
	}

	result := m.viewports.RenderFullscreenViewport(fullscreenXpath, content, renderCounter, footerHeaderHeight)
	buf.AppendFrom(result)
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
