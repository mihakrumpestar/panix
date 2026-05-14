package flow

import (
	"math"
	"strconv"
	"time"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
)

func (pf *PhaseFlow) renderInto(buf *buffer.LinesBuf) {
	numPhases := len(pf.phases)
	available := pf.width - arrowCellWidth*(numPhases-1)

	if available <= 0 {
		return
	}

	base := available / numPhases
	extra := available % numPhases

	var colWidths [16]int
	for i := range numPhases {
		colWidths[i] = base
		if i < extra {
			colWidths[i]++
		}
	}

	pf.arrowBuf.Reset()
	pf.styles.Arrow.Width(1).Align(style.Center).RenderLineInto(pf.arrowBuf, pf.styles.PhaseArrow)

	for len(pf.cellBufs) < numPhases {
		pf.cellBufs = append(pf.cellBufs, buffer.NewLinesBuf())
	}

	pf.parts = pf.parts[:0]

	for idx := range pf.phases {
		phaseData := PhaseData{}
		if idx < len(pf.data) {
			phaseData = pf.data[idx]
		}

		if idx > 0 {
			pf.parts = append(pf.parts, pf.arrowBuf)
		}

		pf.parts = append(pf.parts, pf.buildCellBuf(idx, phaseData, colWidths[idx], idx == pf.selectedIndex))
	}

	pf.joinBuf.Reset()
	style.JoinHorizontalBufs(pf.joinBuf, style.Top, pf.parts...)
	pf.centerLinesInto(buf, pf.joinBuf, pf.width)
}

func (pf *PhaseFlow) buildCellBuf(idx int, data PhaseData, colWidth int, isSelected bool) *buffer.LinesBuf {
	state := determineState(data)
	pillBuf := pf.createAnimatedGradientPill(pf.phaseNames[idx], state)
	pillWidth := style.MaxLineWidth(pillBuf)

	pf.cellBuf.Reset()
	pf.centerLinesInto(pf.cellBuf, pillBuf, pillWidth)

	pf.writeStatusLines(pf.cellBuf, data, pillWidth, isSelected)

	if pf.zonePrefix != "" && idx < len(pf.zoneIDs) {
		pf.zonedBuf.Reset()
		pf.zoneIDs[idx].MarkLines(pf.cellBuf, pf.zonedBuf)
		pf.cellBuf, pf.zonedBuf = pf.zonedBuf, pf.cellBuf
	}

	dst := pf.cellBufs[idx]
	dst.Reset()
	pf.centerLinesInto(dst, pf.cellBuf, colWidth)

	return dst
}

func (pf *PhaseFlow) centerLinesInto(dst *buffer.LinesBuf, src *buffer.LinesBuf, targetWidth int) {
	for i := range src.Len() {
		line := src.Line(i)
		pad := targetWidth - style.CellWidth(line)

		if pad <= 0 {
			dst.WriteLine(line)
		} else {
			l := pad / 2
			dst.WriteLine(style.PaddingBytes(l), line, style.PaddingBytes(pad-l))
		}
	}
}

// renderCounts writes non-zero counts styled into pf.statusBuf.
func (pf *PhaseFlow) renderCounts(data PhaseData, ss StatusStyles) {
	pf.renderCount(data.Running, ss.Running)
	pf.renderCount(data.Failed, ss.Failed)
	pf.renderCount(data.Done, ss.Done)
}

func (pf *PhaseFlow) renderCount(val int, sty style.Style) {
	if val <= 0 {
		return
	}

	pf.lineBuf.Reset()
	pf.lineBuf.Set(strconv.AppendInt(pf.lineBuf.Bytes()[:0], int64(val), 10)) //nolint:mnd
	sty.RenderLineInto(pf.statusBuf, pf.lineBuf.Bytes())
}

// joinStatusBuf assembles pf.statusBuf lines into pf.statusLine with separators,
// then centers into dst at pillWidth. When selBgStyle is non-zero, it is used
// to render background-colored padding spaces.
func (pf *PhaseFlow) joinStatusBuf(dst *buffer.LinesBuf, sepStyle style.Style, pillWidth int, selBgStyle style.Style) {
	hasBg := len(selBgStyle.RenderLine(nil)) > 0

	if pf.statusBuf.Len() == 0 {
		if hasBg && pillWidth > 0 {
			bgSpaces := selBgStyle.RenderLine(style.PaddingBytes(pillWidth))
			dst.WriteLine(bgSpaces)
		} else {
			dst.WriteLine(nil)
		}

		return
	}

	length := pf.statusBuf.Len()

	pf.statusLine.Reset()
	pf.statusLine.EmptyLine()

	if hasBg {
		contentWidth := 0

		for i := range length {
			w := style.CellWidth(pf.statusBuf.Line(i))

			contentWidth += w
			if i > 0 {
				contentWidth += 1 + style.CellWidth(sepStyle.RenderLine(slashSep))
			}
		}

		pad := pillWidth - contentWidth

		if pad > 0 {
			left := pad / 2

			bgSpace := selBgStyle.RenderLine([]byte(" "))
			for range left {
				pf.statusLine.AppendToLine(bgSpace)
			}

			pf.appendStatusCounts(sepStyle, length)

			for range pad - left {
				pf.statusLine.AppendToLine(bgSpace)
			}

			pf.centerLinesInto(dst, pf.statusLine, pillWidth)

			return
		}
	}

	pf.appendStatusCounts(sepStyle, length)
	pf.centerLinesInto(dst, pf.statusLine, pillWidth)
}

func (pf *PhaseFlow) appendStatusCounts(sepStyle style.Style, n int) {
	for i := range n {
		if i > 0 {
			sepStyle.RenderAppend(pf.statusLine, slashSep)
		}

		pf.statusLine.AppendToLine(pf.statusBuf.Line(i))
	}
}

func (pf *PhaseFlow) writeStatusLines(dst *buffer.LinesBuf, data PhaseData, pillWidth int, isSelected bool) {
	pf.statusBuf.Reset()

	if isSelected {
		pf.renderCounts(data, pf.styles.StatusSel)
		pf.joinStatusBuf(dst, pf.styles.SelBgStyle, pillWidth, pf.styles.SelBgStyle)

		return
	}

	pf.renderCounts(data, pf.styles.Status)
	pf.joinStatusBuf(dst, pf.styles.StatusSeparator, pillWidth, style.Style{})
}

func (pf *PhaseFlow) createAnimatedGradientPill(text []byte, state PhaseState) *buffer.LinesBuf {
	now := time.Now()
	lastTime := pf.animation.lastTime.Load()

	if lastTime.IsZero() || now.Sub(lastTime) >= animInterval {
		pf.animation.lastTime.Store(now)

		t := float64(now.UnixNano()%int64(gradientCycleTime)) / float64(gradientCycleTime)
		progress := math.Sin(t*2*math.Pi)*animAmplitude + animAmplitude
		pf.animation.progress.Store(math.Float64bits(progress))
	}

	progress := math.Float64frombits(pf.animation.progress.Load())
	gradient := pf.styles.gradientForState(state)

	finalColor := gradient.Dark.BlendLuv(gradient.Light, progress)
	hexColor := style.Color(finalColor.Hex())

	pf.pillBuf.Reset()
	pf.styles.Pill.Background(hexColor).RenderLineInto(pf.pillBuf, text)

	return pf.pillBuf
}

func (pf *PhaseFlow) animationNeedsUpdate() bool {
	lastTime := pf.animation.lastTime.Load()

	return lastTime.IsZero() || time.Since(lastTime) >= animInterval
}

func determineState(data PhaseData) PhaseState {
	if data.Running > 0 {
		return StateRunning
	}

	if data.Failed > 0 {
		return StateFailed
	}

	if data.Done > 0 {
		return StateDone
	}

	return Idle
}
