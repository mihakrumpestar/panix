package flow

import (
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"go.uber.org/atomic"
)

const (
	gradientCycleTime = 4 * time.Second
	animInterval      = 100 * time.Millisecond
	animAmplitude     = 0.5
	arrowCellWidth    = 1
)

var slashSep = []byte("/")

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

// StatusStyles holds the three count styles (normal and selected variants).
type StatusStyles struct {
	Running style.Style
	Failed  style.Style
	Done    style.Style
}

// Styles holds all style configuration for PhaseFlow rendering.
type Styles struct {
	GradientRunning GradientPair
	GradientFailed  GradientPair
	GradientDone    GradientPair
	GradientDefault GradientPair
	Pill            style.Style

	Status          StatusStyles
	StatusSel       StatusStyles
	StatusSeparator style.Style

	Arrow       style.Style
	PhaseArrow  []byte
	SelectionBg style.Color

	SelBgStyle style.Style
}

// InitSelectedStyles pre-computes the selected-background variants of status
// styles and the SelectionBg style. Called automatically by Styles().
func (s *Styles) InitSelectedStyles() {
	background := s.SelectionBg
	s.StatusSel = StatusStyles{
		Running: s.Status.Running.Background(background),
		Failed:  s.Status.Failed.Background(background),
		Done:    s.Status.Done.Background(background),
	}
	s.SelBgStyle = style.NewStyle().Background(background)
}

func (s *Styles) gradientForState(state PhaseState) GradientPair {
	switch state {
	case StateRunning:
		return s.GradientRunning
	case StateFailed:
		return s.GradientFailed
	case StateDone:
		return s.GradientDone
	default:
		return s.GradientDefault
	}
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
type PhaseFlow struct {
	width         int
	phases        []string
	phaseNames    [][]byte
	data          []PhaseData
	styles        Styles
	selectedIndex int
	zonePrefix    string
	zoneIDs       []zeroterm.ZoneID

	cacheData   []PhaseData
	cacheWidth  int
	cacheSelIdx int
	outDirty    bool

	content   *buffer.LinesBuf
	animation animationState

	// Persistent scratch buffers for zero-alloc rebuilds.
	pillBuf    *buffer.LinesBuf
	statusBuf  *buffer.LinesBuf
	cellBuf    *buffer.LinesBuf
	zonedBuf   *buffer.LinesBuf
	arrowBuf   *buffer.LinesBuf
	joinBuf    *buffer.LinesBuf
	lineBuf    *buffer.LineBuf
	statusLine *buffer.LinesBuf
	cellBufs   []*buffer.LinesBuf
	parts      []*buffer.LinesBuf
}

// New creates a PhaseFlow with no phases and no selection.
func New() *PhaseFlow {
	return &PhaseFlow{
		selectedIndex: -1,
		outDirty:      true,
		content:       buffer.NewLinesBuf(),
		pillBuf:       buffer.NewLinesBuf(),
		statusBuf:     buffer.NewLinesBuf(),
		cellBuf:       buffer.NewLinesBuf(),
		zonedBuf:      buffer.NewLinesBuf(),
		arrowBuf:      buffer.NewLinesBuf(),
		joinBuf:       buffer.NewLinesBuf(),
		lineBuf:       buffer.NewLineBuf(),
		statusLine:    buffer.NewLinesBuf(),
	}
}
