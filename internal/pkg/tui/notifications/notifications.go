package notifications

import (
	"fmt"
	"image/color"
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
)

const (
	duration     = 3 * time.Second
	fadeStart    = 1 * time.Second
	tickInterval = 250 * time.Millisecond

	fadeFactor = 0.4
	rgbaShift  = 8
)

type notificationTickMsg struct{}

type Notification struct {
	text    string
	fgR     uint8
	fgG     uint8
	fgB     uint8
	started time.Time
}

func New() *Notification { return &Notification{} }

func (n *Notification) Set(text string, sty style.Style) render.Cmd {
	n.text, n.started = text, time.Now()
	n.fgR, n.fgG, n.fgB = 180, 180, 180

	fg := sty.GetForeground()
	if fg != nil {
		r, g, b, _ := fg.RGBA()

		// #nosec G115 -- rgba values are 0-65535, >>8 safely converts to 0-255 range
		n.fgR, n.fgG, n.fgB = uint8(r>>rgbaShift), uint8(g>>rgbaShift), uint8(b>>rgbaShift)
	}

	return render.TickCmd(tickInterval, func(time.Time) render.Msg { return notificationTickMsg{} })
}

func (n *Notification) Update(msg render.Msg) render.Cmd {
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

	return render.TickCmd(tickInterval, func(time.Time) render.Msg { return notificationTickMsg{} })
}

func (n *Notification) Clear() {
	n.text = ""
	n.started = time.Time{}
}

func (n *Notification) View(baseStyle style.Style) (string, int) {
	if n.isExpired() {
		return "", 0
	}

	fg := n.fadedColor()
	box := style.NewStyle().
		Border(style.RoundedBorder()).
		BorderForeground(fg).
		Render(n.render(baseStyle))

	return box + "\n", style.CellWidth(box)
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

func (n *Notification) fadedColor() color.Color {
	elapsed := time.Since(n.started)
	if elapsed < fadeStart {
		return style.Color(fmt.Sprintf("#%02x%02x%02x", n.fgR, n.fgG, n.fgB))
	}

	progress := min(float64(elapsed-fadeStart)/float64(duration-fadeStart), 1.0)
	factor := 1.0 - (progress * fadeFactor)

	return style.Color(fmt.Sprintf("#%02x%02x%02x",
		uint8(float64(n.fgR)*factor),
		uint8(float64(n.fgG)*factor),
		uint8(float64(n.fgB)*factor)))
}
