// Derived from charm.land/bubbletea/v2. See pkg/tui/LICENSE.charmbracelet.

package zeroterm

import "github.com/mihakrumpestar/panix/pkg/buffer"

type MouseButton uint8

const (
	MouseLeft MouseButton = iota
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
)

type Msg any

type KeyPressMsg struct {
	Key string
}

func (m KeyPressMsg) String() string { return m.Key }

type MouseClickMsg struct {
	X      int
	Y      int
	Button MouseButton
	Lines  *buffer.LinesBufDiff
}

type MouseWheelMsg struct {
	X      int
	Y      int
	Button MouseButton
}

type WindowSizeMsg struct {
	Width  int
	Height int
}

type QuitMsg struct{}

type PostRenderMsg struct{}

func QuitCmd() Msg { return QuitMsg{} }

type Cmd func() Msg

type Model interface {
	Init() []Cmd
	Update(msg Msg) Cmd
	// Render writes the screen lines into the LinesBuffer.
	// renderCounter is a monotonically increasing counter managed by the
	// Program; callers use it for change detection (e.g., viewport
	// content versioning) without tracking it themselves.
	Render(buf *buffer.LinesBufDiff, renderCounter uint64)
}
