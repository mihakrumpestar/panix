package zeroterm

import (
	"testing"
)

func TestKeyPressMsg(t *testing.T) {
	t.Parallel()

	msg := KeyPressMsg{Key: "enter"}
	if msg.String() != "enter" {
		t.Errorf("KeyPressMsg.String() = %q, want %q", msg.String(), "enter")
	}
}

func TestMouseClickMsg(t *testing.T) {
	t.Parallel()

	msg := MouseClickMsg{X: 10, Y: 5, Button: MouseLeft}
	if msg.X != 10 || msg.Y != 5 || msg.Button != MouseLeft {
		t.Error("MouseClickMsg fields mismatch")
	}
}

func TestMouseWheelMsg(t *testing.T) {
	t.Parallel()

	msg := MouseWheelMsg{X: 10, Y: 5, Button: MouseWheelUp}
	if msg.Button != MouseWheelUp {
		t.Error("MouseWheelMsg button mismatch")
	}
}

func TestWindowSizeMsg(t *testing.T) {
	t.Parallel()

	msg := WindowSizeMsg{Width: 80, Height: 24}
	if msg.Width != 80 || msg.Height != 24 {
		t.Error("WindowSizeMsg fields mismatch")
	}
}

func TestQuitMsg(t *testing.T) {
	t.Parallel()

	msg := QuitMsg{}
	_ = msg
}

func TestQuitCmd(t *testing.T) {
	t.Parallel()

	msg := QuitCmd()
	if _, ok := msg.(QuitMsg); !ok {
		t.Error("QuitCmd should return QuitMsg")
	}
}

type mockModel struct{}

func (m *mockModel) Init() []Cmd          { return nil }
func (m *mockModel) Update(msg Msg) []Cmd { return nil }
func (m *mockModel) Render() []string     { return nil }

func TestModelInterface(t *testing.T) {
	t.Parallel()

	var _ Model = &mockModel{}
}

func TestCmdFunc(t *testing.T) {
	t.Parallel()

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

func TestMouseButtonValues(t *testing.T) {
	t.Parallel()

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
