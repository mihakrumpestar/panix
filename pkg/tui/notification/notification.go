package notification

import (
	"fmt"
	"time"

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
}

func New(defaultColor style.Color) *Notification {
	return &Notification{defaultColor: defaultColor}
}

func (n *Notification) Set(text string, c style.Color) zeroterm.Cmd {
	n.text, n.started = text, time.Now()
	n.currentColor = n.defaultColor

	if c != "" {
		n.currentColor = c
	}

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

	return zeroterm.TickCmd(tickInterval, func(time.Time) zeroterm.Msg { return notificationTickMsg{} })
}

func (n *Notification) Clear() {
	n.text = ""
	n.started = time.Time{}
}

func (n *Notification) View(baseStyle style.Style) string {
	if n.isExpired() {
		return ""
	}

	fg := n.fadedColor()
	box := style.NewStyle().
		Border(style.RoundedBorder()).
		BorderForeground(fg).
		Padding(0, 1).
		Render(n.render(baseStyle))

	return box
}

func (n *Notification) render(baseStyle style.Style) string {
	if n.isExpired() {
		return ""
	}

	return baseStyle.Foreground(n.fadedColor()).Render(n.text)
}

func (n *Notification) isExpired() bool {
	return n.text == "" || time.Since(n.started) >= duration
}

func (n *Notification) fadedColor() style.Color {
	elapsed := time.Since(n.started)
	r, g, b := style.ColorToRGB8(n.currentColor)

	if elapsed < fadeStart {
		return style.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
	}

	progress := min(float64(elapsed-fadeStart)/float64(duration-fadeStart), 1.0)
	factor := 1.0 - (progress * fadeFactor)

	return style.Color(fmt.Sprintf("#%02x%02x%02x",
		uint8(float64(r)*factor),
		uint8(float64(g)*factor),
		uint8(float64(b)*factor)))
}
