package zeroterm

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

func QuitCmd() Msg { return QuitMsg{} }

type Cmd func() Msg

type Model interface {
	Init() []Cmd
	Update(msg Msg) []Cmd
	// Render returns the screen lines as ANSI strings (one per line).
	// Returns nil if nothing changed since the last call (cache hit).
	Render() []string
}
