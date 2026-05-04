package flow

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
	"go.uber.org/atomic"
)

const (
	gradientCycleTime = 4 * time.Second
	animInterval      = 100 * time.Millisecond
	animAmplitude     = 0.5
	arrowCellWidth    = 1
)

// PhaseState determines which gradient colors to use for a phase pill.
type PhaseState int

const (
	Idle PhaseState = iota
	StateRunning
	StateFailed
	StateDone
)

// GradientPair holds two colorful.Color values for animating between.
type GradientPair struct {
	Dark  colorful.Color
	Light colorful.Color
}

// Styles holds all style configuration for PhaseFlow rendering.
type Styles struct {
	GradientRunning GradientPair
	GradientFailed  GradientPair
	GradientDone    GradientPair
	GradientDefault GradientPair
	Pill            style.Style

	StatusRunning   style.Style
	StatusFailed    style.Style
	StatusDone      style.Style
	StatusSeparator style.Style

	Arrow      style.Style
	PhaseArrow string
	SelectionBg style.Color
}

// PhaseData holds the per-phase counts shown beneath the pill.
type PhaseData struct {
	Running int
	Failed  int
	Done    int
}

type animationState struct {
	progress atomic.Uint64
	lastTime atomic.Time
}

// PhaseFlow is a horizontal phase-flow component that renders phase name pills
// with animated gradient backgrounds and status count lines beneath them.
// Phases are distributed evenly across the given width, separated by arrows.
type PhaseFlow struct {
	width         int
	phases        []string
	data          []PhaseData
	styles        Styles
	selectedIndex int
	zonePrefix    string

	cacheData   []PhaseData
	cacheWidth  int
	cacheSelIdx int
	cacheResult string

	animation animationState
}

// New creates a PhaseFlow with no phases and no selection.
func New() *PhaseFlow {
	return &PhaseFlow{
		selectedIndex: -1,
	}
}

// Width sets the total rendering width.
func (pf *PhaseFlow) Width(w int) *PhaseFlow {
	if w == pf.width {
		return pf
	}

	pf.width = w
	pf.cacheResult = ""

	return pf
}

// Phases sets the phase names (headers). The number of phases determines the
// column count. Data is reset to zero-filled PhaseData of matching length.
func (pf *PhaseFlow) Phases(names ...string) *PhaseFlow {
	pf.phases = names
	pf.data = make([]PhaseData, len(names))
	pf.cacheResult = ""

	return pf
}

// Styles sets the rendering styles.
func (pf *PhaseFlow) Styles(s Styles) *PhaseFlow {
	pf.styles = s
	pf.cacheResult = ""

	return pf
}

// SetZonePrefix sets the prefix used for zone markers (e.g. "phase-status").
func (pf *PhaseFlow) SetZonePrefix(prefix string) *PhaseFlow {
	pf.zonePrefix = prefix

	return pf
}

// SetData replaces the per-phase count data. The slice is copied internally.
func (pf *PhaseFlow) SetData(data []PhaseData) {
	pf.data = append([]PhaseData(nil), data...)
}

// SelectedIndex returns the currently selected phase index, or -1 if none.
func (pf *PhaseFlow) SelectedIndex() int {
	return pf.selectedIndex
}

// Deselect clears the selection.
func (pf *PhaseFlow) Deselect() {
	if pf.selectedIndex == -1 {
		return
	}

	pf.selectedIndex = -1
	pf.cacheResult = ""
}

// Reset clears the selection.
func (pf *PhaseFlow) Reset() {
	pf.Deselect()
}

// HandleNavigation processes left/right key input. Returns true if the
// selection changed. When hasActiveInnerViewport is true, navigation is
// ignored.
func (pf *PhaseFlow) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(pf.phases) == 0 {
		return false
	}

	switch key {
	case "left":
		if pf.selectedIndex > 0 {
			pf.selectedIndex--
			pf.cacheResult = ""

			return true
		}

		if pf.selectedIndex < 0 {
			pf.selectedIndex = 0
			pf.cacheResult = ""

			return true
		}
	case "right":
		if pf.selectedIndex < 0 {
			pf.selectedIndex = 0
			pf.cacheResult = ""

			return true
		}

		if pf.selectedIndex < len(pf.phases)-1 {
			pf.selectedIndex++
			pf.cacheResult = ""

			return true
		}
	}

	return false
}

// HandleMouseClick processes a mouse click. Returns true if the selection
// changed. Clicking outside any phase zone deselects.
func (pf *PhaseFlow) HandleMouseClick(msg render.MouseClickMsg) bool {
	if len(pf.phases) == 0 {
		return false
	}

	lines := render.CurrentLines()

	if msg.Y < 0 || msg.Y >= len(lines) {
		if pf.selectedIndex >= 0 {
			pf.selectedIndex = -1
			pf.cacheResult = ""

			return true
		}

		return false
	}

	for idx := range pf.phases {
		if pf.zonePrefix == "" {
			continue
		}

		zoneName := fmt.Sprintf("%s-%d", pf.zonePrefix, idx)
		if render.IsZoneAtLine(lines[msg.Y], msg.X, zoneName) {
			if pf.selectedIndex != idx {
				pf.selectedIndex = idx
				pf.cacheResult = ""

				return true
			}

			return false
		}
	}

	if pf.selectedIndex >= 0 {
		pf.selectedIndex = -1
		pf.cacheResult = ""

		return true
	}

	return false
}

// String renders the phase flow and returns the result. Uses a cache: returns
// the previous result if data, width, and selection haven't changed and the
// animation timer hasn't elapsed.
func (pf *PhaseFlow) String() string {
	if pf.width == 0 || len(pf.phases) == 0 {
		return ""
	}

	if pf.cacheResult != "" &&
		pf.cacheWidth == pf.width &&
		pf.cacheSelIdx == pf.selectedIndex &&
		phaseDataEqual(pf.cacheData, pf.data) &&
		!pf.animationNeedsUpdate() {
		return pf.cacheResult
	}

	result := pf.render()

	pf.cacheData = append([]PhaseData(nil), pf.data...)
	pf.cacheWidth = pf.width
	pf.cacheSelIdx = pf.selectedIndex
	pf.cacheResult = result

	return result
}

func (pf *PhaseFlow) render() string {
	n := len(pf.phases)

	totalArrowWidth := arrowCellWidth * (n - 1)
	available := pf.width - totalArrowWidth

	if available <= 0 {
		return ""
	}

	base := available / n
	extra := available % n

	colWidths := make([]int, n)
	for i := range n {
		colWidths[i] = base
		if i < extra {
			colWidths[i]++
		}
	}

	cells := make([]string, n)

	for i := range pf.phases {
		d := PhaseData{}
		if i < len(pf.data) {
			d = pf.data[i]
		}

		isSelected := i == pf.selectedIndex
		cells[i] = pf.buildCell(pf.phases[i], d, colWidths[i], isSelected, i)
	}

	arrowStyled := pf.styles.Arrow.Width(1).Align(style.Center).Render(pf.styles.PhaseArrow)

	parts := make([]string, 0, 2*n-1)

	for i, cell := range cells {
		if i > 0 {
			parts = append(parts, arrowStyled)
		}

		parts = append(parts, cell)
	}

	// Top alignment so arrows appear on the phase-name row, not the status row.
	row := style.JoinHorizontal(style.Top, parts...)
	row = style.NewStyle().Width(pf.width).Align(style.Center).Render(row)

	return row
}

func (pf *PhaseFlow) buildCell(name string, data PhaseData, colWidth int, isSelected bool, idx int) string {
	state := determineState(data)
	pill := pf.createAnimatedGradient(name, state)

	pillWidth := style.CellWidth(pill)

	statusContent := pf.buildStatusLine(data, isSelected)

	if isSelected {
		contentWidth := style.CellWidth(statusContent)
		pad := pillWidth - contentWidth
		if pad > 0 {
			left := pad / 2
			right := pad - left
			bgSpace := style.NewStyle().Background(pf.styles.SelectionBg).Render(" ")
			statusContent = strings.Repeat(bgSpace, left) + statusContent + strings.Repeat(bgSpace, right)
		}
	}

	// Build both rows at pillWidth so the zone matches the visual pill span.
	pillRow := style.NewStyle().Width(pillWidth).Align(style.Center).Render(pill)
	statusRow := style.NewStyle().Width(pillWidth).Align(style.Center).Render(statusContent)

	cell := pillRow + "\n" + statusRow
	if pf.zonePrefix != "" {
		cell = render.Mark(fmt.Sprintf("%s-%d", pf.zonePrefix, idx), cell)
	}

	// Center the zoned cell into the column.
	cell = style.NewStyle().Width(colWidth).Align(style.Center).Render(cell)

	return cell
}

func (pf *PhaseFlow) createAnimatedGradient(text string, state PhaseState) string {
	now := time.Now()
	lastTime := pf.animation.lastTime.Load()

	if lastTime.IsZero() || now.Sub(lastTime) >= animInterval {
		pf.animation.lastTime.Store(now)

		t := float64(now.UnixNano()%int64(gradientCycleTime)) / float64(gradientCycleTime)
		progress := math.Sin(t*2*math.Pi)*animAmplitude + animAmplitude

		pf.animation.progress.Store(math.Float64bits(progress))
	}

	progress := math.Float64frombits(pf.animation.progress.Load())

	var gp GradientPair

	switch state {
	case StateRunning:
		gp = pf.styles.GradientRunning
	case StateFailed:
		gp = pf.styles.GradientFailed
	case StateDone:
		gp = pf.styles.GradientDone
	default:
		gp = pf.styles.GradientDefault
	}

	finalColor := gp.Dark.BlendLuv(gp.Light, progress)

	return pf.styles.Pill.Background(style.Color(finalColor.Hex())).Render(text)
}

func (pf *PhaseFlow) buildStatusLine(data PhaseData, isSelected bool) string {
	if isSelected {
		selBg := pf.styles.SelectionBg
		sep := style.NewStyle().Background(selBg).Render("/")

		var parts []string
		if data.Running > 0 {
			parts = append(parts, pf.styles.StatusRunning.Background(selBg).Render(strconv.Itoa(data.Running)))
		}
		if data.Failed > 0 {
			parts = append(parts, pf.styles.StatusFailed.Background(selBg).Render(strconv.Itoa(data.Failed)))
		}
		if data.Done > 0 {
			parts = append(parts, pf.styles.StatusDone.Background(selBg).Render(strconv.Itoa(data.Done)))
		}

		return strings.Join(parts, sep)
	}

	var parts []string

	if data.Running > 0 {
		parts = append(parts, pf.styles.StatusRunning.Render(strconv.Itoa(data.Running)))
	}

	if data.Failed > 0 {
		parts = append(parts, pf.styles.StatusFailed.Render(strconv.Itoa(data.Failed)))
	}

	if data.Done > 0 {
		parts = append(parts, pf.styles.StatusDone.Render(strconv.Itoa(data.Done)))
	}

	return strings.Join(parts, pf.styles.StatusSeparator.Render("/"))
}

func (pf *PhaseFlow) animationNeedsUpdate() bool {
	lastTime := pf.animation.lastTime.Load()
	if lastTime.IsZero() {
		return true
	}

	return time.Since(lastTime) >= animInterval
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

func phaseDataEqual(a, b []PhaseData) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
