package zeroterm

import (
	"testing"

	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestKeyPressMsg(t *testing.T) {
	assert.Equal(t, "enter", KeyPressMsg{Key: "enter"}.String())
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMouseClickMsg(t *testing.T) {
	msg := MouseClickMsg{X: 10, Y: 5, Button: MouseLeft}
	assert.Equal(t, 10, msg.X)
	assert.Equal(t, 5, msg.Y)
	assert.Equal(t, MouseLeft, msg.Button)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMouseWheelMsg(t *testing.T) {
	assert.Equal(t, MouseWheelUp, MouseWheelMsg{Button: MouseWheelUp}.Button)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestWindowSizeMsg(t *testing.T) {
	msg := WindowSizeMsg{Width: 80, Height: 24}
	assert.Equal(t, 80, msg.Width)
	assert.Equal(t, 24, msg.Height)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestQuitMsg(t *testing.T) {
	_ = QuitMsg{}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestQuitCmd(t *testing.T) {
	_, ok := QuitCmd().(QuitMsg)
	assert.True(t, ok, "QuitCmd should return QuitMsg")
}

type mockModel struct{}

func (m *mockModel) Init() []Cmd                               { return nil }
func (m *mockModel) Update(msg Msg) Cmd                        { return nil }
func (m *mockModel) Render(buf *buffer.LinesBufDiff, _ uint64) {}

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

	assert.True(t, called, "Cmd function should have been called")

	k, ok := result.(KeyPressMsg)
	assert.True(t, ok)
	assert.Equal(t, "a", k.Key)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestMouseButtonValues(t *testing.T) {
	assert.Equal(t, 0, int(MouseLeft))
	assert.Equal(t, 1, int(MouseMiddle))
	assert.Equal(t, 2, int(MouseRight))
	assert.Equal(t, 3, int(MouseWheelUp))
	assert.Equal(t, 4, int(MouseWheelDown))
}
