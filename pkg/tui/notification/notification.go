package notification

import (
	"bytes"
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

const (
	defaultDuration  = 3 * time.Second
	defaultFadeStart = 1 * time.Second
	tickInterval     = 150 * time.Millisecond

	fadeFactor = 0.4
)

// SeverityLevel selects a notification's presentation from Styles.
type SeverityLevel string

const (
	LevelInfo    SeverityLevel = "info"
	LevelSuccess SeverityLevel = "success"
	LevelWarning SeverityLevel = "warning"
	LevelError   SeverityLevel = "error"
)

// LevelStyle is how one notification level renders.
type LevelStyle struct {
	Marker      string      // optional marker cell, e.g. "! " (empty = none)
	MarkerStyle style.Style // marker styling (e.g. warning color)
	TextStyle   style.Style // message overrides on top of the base style
	FrameColor  style.Color // notification frame/foreground color
}

// Styles is the theme-provided notification styling.
type Styles struct {
	DefaultFg style.Color
	Base      style.Style
	Border    style.Border
	Info      LevelStyle
	Success   LevelStyle
	Warning   LevelStyle
	Error     LevelStyle
}

type notificationTickMsg struct{}

type Notification struct {
	text    string
	level   SeverityLevel
	styles  Styles
	started time.Time

	// A duration of 0 means the notification never expires (persistent) and
	// does not fade.
	duration  time.Duration
	fadeStart time.Duration

	// markerColumnWidth is the cell width of the optional marker column,
	// derived from the widest level marker so messages align across
	// notifications that carry a marker.
	markerColumnWidth int

	buf *buffer.LinesBufVer
}

// New creates a notification with the given styles. A zero Border falls back
// to the rounded preset so a notification always draws a frame.
func New(styles Styles) *Notification {
	if len(styles.Border.Vertical) == 0 {
		styles.Border = style.RoundedBorder()
	}

	notification := &Notification{
		duration:  defaultDuration,
		fadeStart: defaultFadeStart,
		buf:       buffer.NewLinesBufVer(),
	}
	notification.ApplyTheme(styles)

	return notification
}

// ApplyTheme replaces the styling and re-renders any active notification so it
// matches the new theme immediately (persistent notifications never tick).
func (n *Notification) ApplyTheme(styles Styles) {
	n.styles = styles
	n.markerColumnWidth = widestMarker(styles)
	n.render()
}

// widestMarker returns the widest level marker in cells so the optional marker
// column aligns every level and no marker is truncated.
func widestMarker(styles Styles) int {
	return max(
		style.CellWidth([]byte(styles.Info.Marker)),
		style.CellWidth([]byte(styles.Success.Marker)),
		style.CellWidth([]byte(styles.Warning.Marker)),
		style.CellWidth([]byte(styles.Error.Marker)),
	)
}

// Set shows a transient notification at the given level.
func (n *Notification) Set(level SeverityLevel, text string) zeroterm.Cmd {
	return n.set(level, text, defaultDuration, defaultFadeStart)
}

// SetPersistent shows the notification until Clear is called: it never
// expires and does not fade. The returned tick renders the new content once;
// no further ticking is needed for a persistent notification.
func (n *Notification) SetPersistent(level SeverityLevel, text string) zeroterm.Cmd {
	return n.set(level, text, 0, 0)
}

func (n *Notification) Update(msg zeroterm.Msg) zeroterm.Cmd {
	_, ok := msg.(notificationTickMsg)
	if !ok {
		return nil
	}

	if n.text == "" {
		return nil
	}

	// Persistent notifications never expire; the initial tick already rendered
	// the content, so no further ticking is needed.
	if n.duration == 0 {
		return nil
	}

	if time.Since(n.started) >= n.duration {
		n.Clear()

		return nil
	}

	n.render()

	return zeroterm.TickCmd(tickInterval, func(time.Time) zeroterm.Msg { return notificationTickMsg{} })
}

func (n *Notification) Clear() {
	n.text = ""
	n.started = time.Time{}
	n.buf.Reset()
}

// Render returns the notification's rendered content. Returns nil when expired.
func (n *Notification) Render() *buffer.LinesBufVer {
	if n.isExpired() {
		return nil
	}

	return n.buf
}

// Version returns the current version of the rendered content.
func (n *Notification) Version() uint64 {
	return n.buf.Version()
}

func (n *Notification) set(level SeverityLevel, text string, duration, fadeStart time.Duration) zeroterm.Cmd {
	n.text, n.level, n.started = text, level, time.Now()
	n.duration, n.fadeStart = duration, fadeStart

	n.render()

	return zeroterm.TickCmd(tickInterval, func(time.Time) zeroterm.Msg { return notificationTickMsg{} })
}

// render draws the optional marker plus styled message in the themed frame. The
// level's frame color (falling back to DefaultFg) fades over the lifetime.
func (n *Notification) render() {
	if n.isExpired() {
		n.buf.Reset()

		return
	}

	level := n.severityLevelStyle()

	frameColor := level.FrameColor
	if frameColor == "" {
		frameColor = n.styles.DefaultFg
	}

	foreground := n.fadedColor(frameColor)

	tmp := buffer.NewLinesBuf()
	n.styles.Base.
		Foreground(foreground).
		Border(n.styles.Border).
		BorderForeground(foreground).
		Padding(0, 1).
		RenderLineInto(tmp, n.content(level, foreground))

	n.buf.CopyFrom(tmp)
	tmp.Release()
}

// content composes the optional marker and message. The message is the base
// style foregrounded by the frame color with the level's TextStyle layered on
// top, so a zero TextStyle keeps the single-color look.
func (n *Notification) content(level LevelStyle, foreground style.Color) []byte {
	textStyle := layerTextStyle(n.styles.Base.Foreground(foreground), level.TextStyle)
	message := textStyle.RenderLine([]byte(n.text))

	if level.Marker == "" {
		return message
	}

	marker := level.MarkerStyle.Width(n.markerColumnWidth).RenderLine([]byte(level.Marker))

	return bytes.Join([][]byte{marker, message}, nil)
}

// severityLevelStyle returns the active level's presentation, defaulting to Info.
func (n *Notification) severityLevelStyle() LevelStyle {
	switch n.level {
	case LevelSuccess:
		return n.styles.Success
	case LevelWarning:
		return n.styles.Warning
	case LevelError:
		return n.styles.Error
	case LevelInfo:
		return n.styles.Info
	default:
		return n.styles.Info
	}
}

// layerTextStyle applies the level's text overrides on top of base: a set
// foreground or background wins, unset properties keep base.
func layerTextStyle(base, overlay style.Style) style.Style {
	if fg := overlay.GetForeground(); fg != "" {
		base = base.Foreground(fg)
	}

	if bg := overlay.GetBackground(); bg != "" {
		base = base.Background(bg)
	}

	return base
}

func (n *Notification) isExpired() bool {
	return n.text == "" || (n.duration > 0 && time.Since(n.started) >= n.duration)
}

func (n *Notification) fadedColor(base style.Color) style.Color {
	if n.duration == 0 {
		return base
	}

	elapsed := time.Since(n.started)
	red, green, blue := style.ColorToRGB8(base)

	if elapsed < n.fadeStart {
		return style.Color(fmt.Sprintf("#%02x%02x%02x", red, green, blue))
	}

	progress := min(float64(elapsed-n.fadeStart)/float64(n.duration-n.fadeStart), 1.0)
	factor := 1.0 - (progress * fadeFactor)

	return style.Color(fmt.Sprintf("#%02x%02x%02x",
		uint8(float64(red)*factor),
		uint8(float64(green)*factor),
		uint8(float64(blue)*factor)))
}
