package zeroterm

import (
	"testing"

	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestKeyPressMsg(t *testing.T) {
	msg := KeyPressMsg{Key: "enter"}
	if msg.String() != "enter" {
		t.Errorf("KeyPressMsg.String() = %q, want %q", msg.String(), "enter")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMouseClickMsg(t *testing.T) {
	msg := MouseClickMsg{X: 10, Y: 5, Button: MouseLeft}
	if msg.X != 10 || msg.Y != 5 || msg.Button != MouseLeft {
		t.Error("MouseClickMsg fields mismatch")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMouseWheelMsg(t *testing.T) {
	msg := MouseWheelMsg{X: 10, Y: 5, Button: MouseWheelUp}
	if msg.Button != MouseWheelUp {
		t.Error("MouseWheelMsg button mismatch")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestWindowSizeMsg(t *testing.T) {
	msg := WindowSizeMsg{Width: 80, Height: 24}
	if msg.Width != 80 || msg.Height != 24 {
		t.Error("WindowSizeMsg fields mismatch")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestQuitMsg(t *testing.T) {
	msg := QuitMsg{}
	_ = msg
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestQuitCmd(t *testing.T) {
	msg := QuitCmd()
	if _, ok := msg.(QuitMsg); !ok {
		t.Error("QuitCmd should return QuitMsg")
	}
}

type mockModel struct{}

func (m *mockModel) Init() []Cmd         { return nil }
func (m *mockModel) Update(msg Msg) Cmd  { return nil }
func (m *mockModel) Render(buf *linesbuffer.LinesBuffer) {}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestModelInterface(t *testing.T) {
	var _ Model = &mockModel{}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestCmdFunc(t *testing.T) {
	var called bool

	cmd := func() Msg {
		called = true

		return KeyPressMsg{Key: "a"}
	}

	result := cmd()

	if !called {
		t.Error("Cmd function should have been called")
	}

	if kpm, ok := result.(KeyPressMsg); !ok || kpm.Key != "a" {
		t.Errorf("Cmd returned %v, want KeyPressMsg{Key: a}", result)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMouseButtonValues(t *testing.T) {
	if MouseLeft != 0 {
		t.Errorf("MouseLeft = %d, want 0", MouseLeft)
	}

	if MouseMiddle != 1 {
		t.Errorf("MouseMiddle = %d, want 1", MouseMiddle)
	}

	if MouseRight != 2 {
		t.Errorf("MouseRight = %d, want 2", MouseRight)
	}

	if MouseWheelUp != 3 {
		t.Errorf("MouseWheelUp = %d, want 3", MouseWheelUp)
	}

	if MouseWheelDown != 4 {
		t.Errorf("MouseWheelDown = %d, want 4", MouseWheelDown)
	}
}
