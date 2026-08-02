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
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/tui/buildlogs"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/profile"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
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
	cachedTree *tree.Node

	// content is a persistent buffer for the main content area.
	// Reused across frames via Reset(); avoids pool buffer loss on GC.
	content *buffer.LinesBuf
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
		viewports: viewports.New(dimensions, conf.Flags.CommandOutputMaxHeight,
			tableS.Border, tableS.SelectionHighlightBackground, tableS.SelectionHighlightBorder),
		statsTable: statstable.New(conf.Fleet, conf.ColorScheme),
		phaseFlow:  phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases),
		cachedTree: tree.NewTree(conf.ColorScheme.Tree.Enumerator, buildlogs.TreeStep),
		content:    buffer.NewLinesBuf(),
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
			mdl.viewMainContentInto(buf, 0, 0, true)
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

	w, err := workflow.NewWorkflow(m.ctx, m.conf)
	if err != nil {
		return []zeroterm.Cmd{func() zeroterm.Msg { return errMsg{err: err} }}
	}

	m.workflow = w

	return []zeroterm.Cmd{
		workflowRunCmd(m.workflow),
		workflowUpdateHookCmd(m.workflow, m.lastWorkflowUpdate),
	}
}

//nolint:cyclop
func (m *model) Update(msg zeroterm.Msg) zeroterm.Cmd {
	var cmd []zeroterm.Cmd

	if m.workflow != nil || m.isSnapshot {
		cmd = append(cmd, m.viewports.Update(msg))

		if m.viewports.ConsumeDirty() {
			m.cachedTree.InvalidateCache()
		}

		cmd = append(cmd, m.footer.Update(msg))
		cmd = append(cmd, m.spinners.Update(msg))
	}

	switch msg := msg.(type) {
	case errMsg:
		cmd = append(cmd, m.handleErrMsgCmd(msg))

		return zeroterm.BatchCmd(cmd...)
	case workflowDoneMsg:
		if msg.workflow != m.workflow {
			return nil
		}

		cmd = append(cmd, m.workflowDoneMsgCmd(msg))

		return zeroterm.BatchCmd(cmd...)
	case restartMsg:
		cmd = append(cmd, m.workflowRestartCmd())
	case retryMsg:
		m.workflowRetry()
	case workflowUpdateHookMsg:
		if msg.workflow == m.workflow {
			m.lastWorkflowUpdate = msg.lastUpdate
			cmd = append(cmd, workflowUpdateHookCmd(m.workflow, m.lastWorkflowUpdate))
		}
	case zeroterm.PostRenderMsg:
		cmd = append(cmd, m.spinners.ProcessPendingTicks())
	case zeroterm.KeyPressMsg:
		cmd = append(cmd, m.HandleKeyInput(msg))
	case zeroterm.MouseClickMsg:
		m.handleMouseClick(msg)
	case zeroterm.WindowSizeMsg:
		m.dimensions.Width = msg.Width
		m.dimensions.Height = msg.Height
		m.cachedTree.InvalidateCache()
	}

	return zeroterm.BatchCmd(cmd...)
}

func (m *model) Render(buf *buffer.LinesBufDiff, renderCounter uint64) {
	if !m.isSnapshot && m.workflow == nil {
		return
	}

	if m.dimensions.Height == 0 || m.dimensions.Width == 0 {
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
		m.viewMainContentInto(buf, renderCounter, headerFooterHeight, false)
	}

	buf.AppendFrom(footer)
}

// viewMainContentInto renders the main content area into buf.
// When finalRender is true, the viewport is rendered unconstrained (full height)
// so the terminal retains the complete history after quit.
func (m *model) viewMainContentInto(buf *buffer.LinesBufDiff, renderCounter uint64, headerFooterHeight int, finalRender bool) {
	if !m.isSnapshot && m.workflow == nil {
		return
	}

	if m.isSnapshot || finalRender {
		m.conf.Fleet.RecalculateCachesOnly(m.conf.Phases)
	} else {
		m.conf.Fleet.Recalculate(m.conf.Phases)
	}

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf, m.statsTable, m.phaseFlow)
	}

	content := m.renderMainContent()

	var viewportHeight int
	if !finalRender {
		viewportHeight = m.dimensions.Height - headerFooterHeight
	}

	buf.AppendFrom(m.viewports.RenderMainViewport(content, renderCounter, viewportHeight))
}

// renderMainContent builds the stats table, phase flow, build logs, error,
// and debug sections into a single buffer.
func (m *model) renderMainContent() *buffer.LinesBuf {
	contentWidth := m.viewports.ContentWidth()
	m.content.Reset()

	if !m.conf.Flags.DryRun {
		if slices.Contains(m.conf.Phases, phase.Inspect) {
			m.content.AppendFrom(m.statsTable.Render(contentWidth))
		}

		m.content.AppendFrom(m.phaseFlow.Render(contentWidth))
	}

	m.buildLogs.RenderInto(m.content, m.cachedTree, m.viewports, m.spinners)

	if m.err != nil {
		m.renderError(m.content)
	}

	if m.conf.Flags.Logging.Debug {
		m.renderDebug(m.content)
	}

	return m.content
}

func (m *model) renderError(content *buffer.LinesBuf) {
	errContent := buffer.NewLinesBuf()
	errContent.EmptyLine()
	errContent.WriteLine([]byte("=== Error ==="))
	errContent.EmptyLine()
	errContent.WriteLine([]byte(m.err.Error()))

	m.conf.ColorScheme.Error.Color.RenderIntoBuf(content, errContent)
	errContent.Release()
}

func (m *model) renderDebug(content *buffer.LinesBuf) {
	content.EmptyLine()
	content.WriteLine([]byte("=== Debug ==="))
	content.EmptyLine()
	content.WriteLine(fmt.Appendf(nil, "terminal - h: %d, w: %d", m.dimensions.Height, m.dimensions.Width))
	content.WriteLine(fmt.Appendf(nil, "header - h: %d", m.header.Len()))
	content.WriteLine(fmt.Appendf(nil, "footer - h: %d", m.footer.Len()))
	content.EmptyLine()

	m.spinners.Debug(content)
	m.viewports.Debug(content)
}

// renderFullscreenViewportInto renders the fullscreen viewport into buf.
func (m *model) renderFullscreenViewportInto(buf *buffer.LinesBufDiff, renderCounter uint64, footerHeaderHeight int) {
	if !m.isSnapshot && m.workflow == nil {
		return
	}

	// Refresh viewport content buffers so the fullscreen viewport shows live data.
	m.content.Reset()
	m.buildLogs.RenderInto(m.content, m.cachedTree, m.viewports, m.spinners)

	fullscreenXpath := m.viewports.GetFullscreenXpath()
	content := m.viewports.GetViewportContent(fullscreenXpath)

	if content == nil || content.Len() == 0 {
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
