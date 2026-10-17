package tui

import (
	"fmt"
	"slices"

	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/tui/buildlogs"
	"github.com/mihakrumpestar/panix/pkg/buffer"
)

func (m *model) Render(buf *buffer.LinesBufDiff, renderCounter uint64) {
	if !m.isSnapshot && m.workflow == nil {
		return
	}

	if m.dimensions.Height == 0 || m.dimensions.Width == 0 {
		return
	}

	if m.buildLogs == nil {
		m.buildLogs = buildlogs.New(m.conf, m.theme, m.statsTable, m.phaseFlow)
	}

	header := m.header.Render(m.dimensions.Width)
	buf.AppendFrom(header)

	// The footer keymap follows the modal context; syncing here covers the frame
	// that opened or closed a modal since the last render.
	m.syncFooterKeyDefs()

	footer := m.footer.Render(m.dimensions.Width)

	headerFooterHeight := header.Len() + footer.Len()

	if m.viewports.IsFullscreen() {
		m.renderFullscreenViewportInto(buf, renderCounter, headerFooterHeight)
	} else {
		m.viewMainContentInto(buf, renderCounter, headerFooterHeight, false)
	}

	buf.AppendFrom(footer)

	// Modal overlays (the theme picker) compose over the frame last; they render
	// nothing when closed.
	m.renderOverlays(buf)
}

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
		m.buildLogs = buildlogs.New(m.conf, m.theme, m.statsTable, m.phaseFlow)
	}

	content := m.renderMainContent()

	var viewportHeight int
	if !finalRender {
		viewportHeight = m.dimensions.Height - headerFooterHeight
	}

	buf.AppendFrom(m.viewports.RenderMainViewport(content, renderCounter, viewportHeight))
}

// renderMainContent builds the stats table, phase flow, build logs, error and
// debug sections into a single buffer.
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

	m.theme.Error.Style.RenderIntoBuf(content, errContent)
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
