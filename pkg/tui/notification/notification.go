package notification

import (
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

type notificationTickMsg struct{}

type Notification struct {
	text         string
	defaultColor style.Color
	currentColor style.Color
	started      time.Time
	baseStyle    style.Style

	// A duration of 0 means the notification never expires (persistent) and
	// does not fade.
	duration  time.Duration
	fadeStart time.Duration

	buf *buffer.LinesBufVer
}

func New(defaultColor style.Color) *Notification {
	return &Notification{
		defaultColor: defaultColor,
		duration:     defaultDuration,
		fadeStart:    defaultFadeStart,
		buf:          buffer.NewLinesBufVer(),
	}
}

func (n *Notification) SetBaseStyle(s style.Style) {
	n.baseStyle = s
}

func (n *Notification) Set(text string, c style.Color) zeroterm.Cmd {
	return n.set(text, c, defaultDuration, defaultFadeStart)
}

// SetPersistent shows the notification until Clear is called: it never
// expires and does not fade. The returned tick renders the new content once;
// no further ticking is needed for a persistent notification.
func (n *Notification) SetPersistent(text string, c style.Color) zeroterm.Cmd {
	return n.set(text, c, 0, 0)
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

func (n *Notification) set(text string, color style.Color, duration, fadeStart time.Duration) zeroterm.Cmd {
	n.text, n.started = text, time.Now()
	n.duration, n.fadeStart = duration, fadeStart
	n.currentColor = n.defaultColor

	if color != "" {
		n.currentColor = color
	}

	n.render()

	return zeroterm.TickCmd(tickInterval, func(time.Time) zeroterm.Msg { return notificationTickMsg{} })
}

func (n *Notification) render() {
	if n.isExpired() {
		n.buf.Reset()

		return
	}

	foreground := n.fadedColor()

	tmp := buffer.NewLinesBuf()
	n.baseStyle.
		Foreground(foreground).
		Border(style.RoundedBorder()).
		BorderForeground(foreground).
		Padding(0, 1).
		RenderLineInto(tmp, []byte(n.text))

	n.buf.CopyFrom(tmp)
	tmp.Release()
}

func (n *Notification) isExpired() bool {
	return n.text == "" || (n.duration > 0 && time.Since(n.started) >= n.duration)
}

func (n *Notification) fadedColor() style.Color {
	if n.duration == 0 {
		return n.currentColor
	}

	elapsed := time.Since(n.started)
	red, green, blue := style.ColorToRGB8(n.currentColor)

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
