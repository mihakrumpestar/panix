package notification

import (
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

const (
	duration     = 3 * time.Second
	fadeStart    = 1 * time.Second
	tickInterval = 150 * time.Millisecond

	fadeFactor = 0.4
)

type notificationTickMsg struct{}

type Notification struct {
	text         string
	defaultColor style.Color
	currentColor style.Color
	started      time.Time
	baseStyle    style.Style

	buf *buffer.LinesBufVer
}

func New(defaultColor style.Color) *Notification {
	return &Notification{
		defaultColor: defaultColor,
		buf:          buffer.NewLinesBufVer(),
	}
}

func (n *Notification) SetBaseStyle(s style.Style) {
	n.baseStyle = s
}

func (n *Notification) Set(text string, c style.Color) zeroterm.Cmd {
	n.text, n.started = text, time.Now()
	n.currentColor = n.defaultColor

	if c != "" {
		n.currentColor = c
	}

	n.render()

	return zeroterm.TickCmd(tickInterval, func(time.Time) zeroterm.Msg { return notificationTickMsg{} })
}

func (n *Notification) Update(msg zeroterm.Msg) zeroterm.Cmd {
	_, ok := msg.(notificationTickMsg)
	if !ok {
		return nil
	}

	if n.text == "" {
		return nil
	}

	if time.Since(n.started) >= duration {
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

func (n *Notification) render() {
	if n.isExpired() {
		n.buf.Reset()

		return
	}

	foreground := n.fadedColor()

	n.buf.Reset()
	n.baseStyle.
		Foreground(foreground).
		Border(style.RoundedBorder()).
		BorderForeground(foreground).
		Padding(0, 1).
		RenderLineInto(n.buf.LinesBuf, []byte(n.text))
}

func (n *Notification) isExpired() bool {
	return n.text == "" || time.Since(n.started) >= duration
}

func (n *Notification) fadedColor() style.Color {
	elapsed := time.Since(n.started)
	red, green, blue := style.ColorToRGB8(n.currentColor)

	if elapsed < fadeStart {
		return style.Color(fmt.Sprintf("#%02x%02x%02x", red, green, blue))
	}

	progress := min(float64(elapsed-fadeStart)/float64(duration-fadeStart), 1.0)
	factor := 1.0 - (progress * fadeFactor)

	return style.Color(fmt.Sprintf("#%02x%02x%02x",
		uint8(float64(red)*factor),
		uint8(float64(green)*factor),
		uint8(float64(blue)*factor)))
}
