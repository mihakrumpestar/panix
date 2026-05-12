package flow

import (
	"fmt"
	"slices"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

func (pf *PhaseFlow) Width(w int) *PhaseFlow {
	if w == pf.width {
		return pf
	}

	pf.width = w
	pf.outDirty = true

	return pf
}

func (pf *PhaseFlow) Phases(names ...string) *PhaseFlow {
	pf.phases = names

	pf.phaseNames = make([][]byte, len(names))
	for i, n := range names {
		pf.phaseNames[i] = []byte(n)
	}

	pf.data = make([]PhaseData, len(names))
	pf.zoneIDs = nil
	pf.outDirty = true

	return pf
}

func (pf *PhaseFlow) Styles(s Styles) *PhaseFlow {
	pf.styles = s
	pf.styles.InitSelectedStyles()
	pf.outDirty = true

	return pf
}

func (pf *PhaseFlow) SetZonePrefix(prefix string) *PhaseFlow {
	pf.zonePrefix = prefix
	pf.zoneIDs = nil

	return pf
}

// SetData replaces the per-phase count data. The slice is copied internally.
// Does not set outDirty — isCacheValid() detects data changes via comparison.
func (pf *PhaseFlow) SetData(data []PhaseData) {
	pf.data = append([]PhaseData(nil), data...)
}

func (pf *PhaseFlow) SelectedIndex() int { return pf.selectedIndex }

func (pf *PhaseFlow) Deselect() {
	if pf.selectedIndex == -1 {
		return
	}

	pf.selectedIndex = -1
	pf.outDirty = true
}

func (pf *PhaseFlow) Reset() { pf.Deselect() }

func (pf *PhaseFlow) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(pf.phases) == 0 {
		return false
	}

	var idx int

	switch key {
	case "left":
		if pf.selectedIndex > 0 {
			idx = pf.selectedIndex - 1

			break
		}

		if pf.selectedIndex < 0 {
			idx = 0

			break
		}

		return false
	case "right":
		if pf.selectedIndex < 0 {
			idx = 0

			break
		}

		if pf.selectedIndex < len(pf.phases)-1 {
			idx = pf.selectedIndex + 1

			break
		}

		return false
	default:
		return false
	}

	pf.selectedIndex = idx
	pf.outDirty = true

	return true
}

func (pf *PhaseFlow) HandleMouseClick(msg zeroterm.MouseClickMsg) bool {
	if len(pf.phases) == 0 {
		return false
	}

	lines := zeroterm.CurrentLines()

	if msg.Y < 0 || msg.Y >= lines.Len() {
		return pf.deselectIfSelected()
	}

	for idx := range pf.phases {
		if pf.zonePrefix == "" {
			continue
		}

		zoneName := fmt.Sprintf("%s-%d", pf.zonePrefix, idx)
		if zeroterm.IsZoneAtLine(lines.Line(msg.Y), msg.X, zoneName) {
			if pf.selectedIndex != idx {
				pf.selectedIndex = idx
				pf.outDirty = true

				return true
			}

			return false
		}
	}

	return pf.deselectIfSelected()
}

func (pf *PhaseFlow) deselectIfSelected() bool {
	if pf.selectedIndex >= 0 {
		pf.selectedIndex = -1
		pf.outDirty = true

		return true
	}

	return false
}

func (pf *PhaseFlow) ensureZoneIDs() {
	if pf.zonePrefix == "" || len(pf.zoneIDs) == len(pf.phases) {
		return
	}

	pf.zoneIDs = make([]uint16, len(pf.phases))
	for idx := range pf.phases {
		pf.zoneIDs[idx] = zeroterm.EnsureZone(fmt.Sprintf("%s-%d", pf.zonePrefix, idx))
	}
}

func (pf *PhaseFlow) isCacheValid() bool {
	return !pf.outDirty &&
		pf.cacheWidth == pf.width &&
		pf.cacheSelIdx == pf.selectedIndex &&
		slices.Equal(pf.cacheData, pf.data) &&
		!pf.animationNeedsUpdate()
}

// Render builds the phase flow and returns the owned content buffer.
// On cache hit returns the existing buffer immediately (zero allocations).
func (pf *PhaseFlow) Render() *buffer.LinesBuf {
	if pf.width == 0 || len(pf.phases) == 0 {
		return pf.content
	}

	if pf.isCacheValid() {
		return pf.content
	}

	pf.ensureZoneIDs()
	pf.content.Reset()

	pf.renderInto(pf.content)

	pf.cacheData = append([]PhaseData(nil), pf.data...)
	pf.cacheWidth = pf.width
	pf.cacheSelIdx = pf.selectedIndex
	pf.outDirty = false

	return pf.content
}

func (pf *PhaseFlow) RenderInto(dst *buffer.LinesBuf) {
	dst.AppendFrom(pf.Render())
}
