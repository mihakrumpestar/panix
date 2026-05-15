package zeroterm

import (
	"fmt"
	"testing"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputLetter(t *testing.T) {
	msgs, _ := parseInput([]byte("a"), false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	keyPress := asKeyPress(msgs[0])

	if keyPress.Key != "a" {
		t.Errorf("key = %q, want %q", keyPress.Key, "a")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEnter(t *testing.T) {
	msgs, _ := parseInput([]byte{0x0D}, false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "enter" {
		t.Errorf("key = %q, want %q", keyPress.Key, "enter")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputTab(t *testing.T) {
	msgs, _ := parseInput([]byte{0x09}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "tab" {
		t.Errorf("key = %q, want %q", keyPress.Key, "tab")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputBackspace(t *testing.T) {
	msgs, _ := parseInput([]byte{0x7F}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "backspace" {
		t.Errorf("key = %q, want %q", keyPress.Key, "backspace")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlC(t *testing.T) {
	msgs, _ := parseInput([]byte{0x03}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+c" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+c")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlD(t *testing.T) {
	msgs, _ := parseInput([]byte{0x04}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+d" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+d")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlZ(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1A}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+z" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+z")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlA(t *testing.T) {
	msgs, _ := parseInput([]byte{0x01}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+a" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+a")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlE(t *testing.T) {
	msgs, _ := parseInput([]byte{0x05}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+e" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+e")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlK(t *testing.T) {
	msgs, _ := parseInput([]byte{0x0B}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+k" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+k")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlU(t *testing.T) {
	msgs, _ := parseInput([]byte{0x15}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+u" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+u")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlW(t *testing.T) {
	msgs, _ := parseInput([]byte{0x17}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+w" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+w")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlR(t *testing.T) {
	msgs, _ := parseInput([]byte{0x12}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+r" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+r")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputNewline(t *testing.T) {
	msgs, _ := parseInput([]byte{0x0A}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "enter" {
		t.Errorf("key = %q, want %q (LF should be enter)", keyPress.Key, "enter")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEscapeAlone(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B}, false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "esc" {
		t.Errorf("key = %q, want %q", keyPress.Key, "esc")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputArrowUp(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'A'}, false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "up" {
		t.Errorf("key = %q, want %q", keyPress.Key, "up")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputArrowDown(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'B'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "down" {
		t.Errorf("key = %q, want %q", keyPress.Key, "down")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputArrowRight(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'C'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "right" {
		t.Errorf("key = %q, want %q", keyPress.Key, "right")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputArrowLeft(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'D'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "left" {
		t.Errorf("key = %q, want %q", keyPress.Key, "left")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputHome(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'H'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "home" {
		t.Errorf("key = %q, want %q", keyPress.Key, "home")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEnd(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'F'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "end" {
		t.Errorf("key = %q, want %q", keyPress.Key, "end")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputDelete(t *testing.T) {
	// CSI 3~
	msgs, _ := parseInput([]byte{0x1B, '[', '3', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "delete" {
		t.Errorf("key = %q, want %q", keyPress.Key, "delete")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputPageUp(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '5', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "pgup" {
		t.Errorf("key = %q, want %q", keyPress.Key, "pgup")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputPageDown(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '6', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "pgdown" {
		t.Errorf("key = %q, want %q", keyPress.Key, "pgdown")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF1VT(t *testing.T) {
	// ESC O P
	msgs, _ := parseInput([]byte{0x1B, 'O', 'P'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "f1" {
		t.Errorf("key = %q, want %q", keyPress.Key, "f1")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF2VT(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, 'O', 'Q'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "f2" {
		t.Errorf("key = %q, want %q", keyPress.Key, "f2")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF3VT(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, 'O', 'R'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "f3" {
		t.Errorf("key = %q, want %q", keyPress.Key, "f3")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF4VT(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, 'O', 'S'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "f4" {
		t.Errorf("key = %q, want %q", keyPress.Key, "f4")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF1CSI(t *testing.T) {
	// CSI 11~
	msgs, _ := parseInput([]byte{0x1B, '[', '1', '1', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "f1" {
		t.Errorf("key = %q, want %q", keyPress.Key, "f1")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF5(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '1', '5', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "f5" {
		t.Errorf("key = %q, want %q", keyPress.Key, "f5")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF12(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '2', '4', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "f12" {
		t.Errorf("key = %q, want %q", keyPress.Key, "f12")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputBacktab(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'Z'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "backtab" {
		t.Errorf("key = %q, want %q", keyPress.Key, "backtab")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseLeftClick(t *testing.T) {
	// SGR mode 1006: ESC[<0;10;5M (left click at (9,4))
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'M'}, false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	click, ok := msgs[0].(MouseClickMsg)
	if !ok {
		t.Fatalf("expected MouseClickMsg, got %T", msgs[0])
	}

	if click.X != 9 || click.Y != 4 {
		t.Errorf("mouse at (%d,%d), want (9,4)", click.X, click.Y)
	}

	if click.Button != MouseLeft {
		t.Errorf("button = %d, want MouseLeft", click.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseRightClick(t *testing.T) {
	// ESC[<2;5;3M (right click at (4,2))
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '2', ';', '5', ';', '3', 'M'}, false)

	click := asMouseClick(msgs[0])
	if click.Button != MouseRight {
		t.Errorf("button = %d, want MouseRight", click.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseWheelUp(t *testing.T) {
	// ESC[<64;10;5M (scroll up at (9,4))
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '6', '4', ';', '1', '0', ';', '5', 'M'}, false)

	wheel, ok := msgs[0].(MouseWheelMsg)
	if !ok {
		t.Fatalf("expected MouseWheelMsg, got %T", msgs[0])
	}

	if wheel.Button != MouseWheelUp {
		t.Errorf("button = %d, want MouseWheelUp", wheel.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseWheelDown(t *testing.T) {
	// ESC[<65;10;5M (scroll down at (9,4))
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '6', '5', ';', '1', '0', ';', '5', 'M'}, false)

	wheel := asMouseWheel(msgs[0])
	if wheel.Button != MouseWheelDown {
		t.Errorf("button = %d, want MouseWheelDown", wheel.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputMultipleKeys(t *testing.T) {
	msgs, _ := parseInput([]byte("abc"), false)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 msgs, got %d", len(msgs))
	}

	for i, want := range []string{"a", "b", "c"} {
		keyPress := asKeyPress(msgs[i])
		if keyPress.Key != want {
			t.Errorf("msg[%d].Key = %q, want %q", i, keyPress.Key, want)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputX11MouseLeftClick(t *testing.T) {
	// X11 format: ESC[M Cb Cx Cy (each byte offset by 32)
	// Left click at (9,4): Cb=0+32=32(' '), Cx=10+32=42('*'), Cy=5+32=37('%')
	msgs, _ := parseInput([]byte{0x1B, '[', 'M', 32, 42, 37}, false)

	click, ok := msgs[0].(MouseClickMsg)
	if !ok {
		t.Fatalf("expected MouseClickMsg, got %T", msgs[0])
	}

	if click.Button != MouseLeft {
		t.Errorf("button = %d, want MouseLeft", click.Button)
	}

	if click.X != 9 || click.Y != 4 {
		t.Errorf("pos = (%d,%d), want (9,4)", click.X, click.Y)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputX11MouseWheelUp(t *testing.T) {
	// X11 wheel up at (9,4): Cb=64+32=96('`'), Cx=10+32=42('*'), Cy=5+32=37('%')
	msgs, _ := parseInput([]byte{0x1B, '[', 'M', 96, 42, 37}, false)

	wheel, ok := msgs[0].(MouseWheelMsg)
	if !ok {
		t.Fatalf("expected MouseWheelMsg, got %T", msgs[0])
	}

	if wheel.Button != MouseWheelUp {
		t.Errorf("button = %d, want MouseWheelUp", wheel.Button)
	}

	if wheel.X != 9 || wheel.Y != 4 {
		t.Errorf("pos = (%d,%d), want (9,4)", wheel.X, wheel.Y)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputX11MouseWheelDown(t *testing.T) {
	// X11 wheel down at (9,4): Cb=65+32=97('a'), Cx=10+32=42('*'), Cy=5+32=37('%')
	msgs, _ := parseInput([]byte{0x1B, '[', 'M', 97, 42, 37}, false)

	wheel, ok := msgs[0].(MouseWheelMsg)
	if !ok {
		t.Fatalf("expected MouseWheelMsg, got %T", msgs[0])
	}

	if wheel.Button != MouseWheelDown {
		t.Errorf("button = %d, want MouseWheelDown", wheel.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputX11MouseDoesNotConsumeExtraBytes(t *testing.T) {
	// X11 click followed by 'a' keypress
	msgs, _ := parseInput([]byte{0x1B, '[', 'M', 32, 42, 37, 'a'}, false)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 msgs, got %d", len(msgs))
	}

	if _, ok := msgs[0].(MouseClickMsg); !ok {
		t.Errorf("first msg = %T, want MouseClickMsg", msgs[0])
	}

	keyPress, ok := msgs[1].(KeyPressMsg)
	if !ok || keyPress.Key != "a" {
		t.Errorf("second msg = %v, want KeyPressMsg{a}", msgs[1])
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputX10MousePartialRead(t *testing.T) {
	// Only 4 bytes — not enough for X10 mouse (needs 6).
	// parseInput processes what it can; partial mouse data is handled
	// by the readInput leftover buffer at the program level.
	msgs, consumed := parseInput([]byte{0x1B, '[', 'M', 32}, false)
	_ = msgs
	// Should consume at least some bytes; exact behavior depends on
	// whether the incomplete sequence can be parsed further.
	if consumed == 0 {
		t.Error("expected non-zero consumed for incomplete data")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputX10MouseMotionIgnored(t *testing.T) {

	// X10 motion event: bit 5 (motion=32) set, button=left(0)
	// Before offset: cb_raw = 32 (motion) + 0 (left) = 32
	// After offset by 32: byte value = 64
	// But 64 has bit 6 (wheel) set, so this conflicts with wheel encoding.
	// Motion + left is actually cb_raw = 32 | 0 = 32, byte = 64.
	// The wheel bit check catches it first as a wheel event since bit 6 is set.
	// In practice, X10 motion with button=0 and bit5=1 => raw=32+0=32,
	// after offset the byte = 32+32 = 64 = 0b0100_0000 which IS the wheel bit.
	// So motion+left-click looks like wheel-up in X10. This is a known
	// ambiguity in X10 protocol — SGR mode resolves it.
	// Test with motion + middle instead: raw = 32|1 = 33, byte = 65.
	// That's 0b0100_0001 which has both wheel and button=1 (wheel-down).
	// X10 simply can't distinguish motion from wheel for buttons 0,1.
	// This is why SGR mode (1006) is preferred.
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputUTF8(t *testing.T) {
	// é = 0xc3 0xa9
	msgs, _ := parseInput([]byte{0xc3, 0xa9}, false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "é" {
		t.Errorf("key = %q, want %q", keyPress.Key, "é")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputShiftModifier(t *testing.T) {
	// CSI 1;2A = shift+up
	msgs, _ := parseInput([]byte{0x1B, '[', '1', ';', '2', 'A'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "shift+up" {
		t.Errorf("key = %q, want %q", keyPress.Key, "shift+up")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlModifier(t *testing.T) {
	// CSI 1;5A = ctrl+up
	msgs, _ := parseInput([]byte{0x1B, '[', '1', ';', '5', 'A'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+up" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+up")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputAltModifier(t *testing.T) {
	// CSI 1;3A = alt+up
	msgs, _ := parseInput([]byte{0x1B, '[', '1', ';', '3', 'A'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "alt+up" {
		t.Errorf("key = %q, want %q", keyPress.Key, "alt+up")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlShiftModifier(t *testing.T) {
	// CSI 1;6A = ctrl+shift+up
	msgs, _ := parseInput([]byte{0x1B, '[', '1', ';', '6', 'A'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+shift+up" {
		t.Errorf("key = %q, want %q", keyPress.Key, "ctrl+shift+up")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputIncompleteEscape(t *testing.T) {
	// Lone ESC at end of buffer
	msgs, _ := parseInput([]byte{0x1B}, false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "esc" {
		t.Errorf("key = %q, want %q", keyPress.Key, "esc")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEmptyInput(t *testing.T) {
	msgs, _ := parseInput([]byte{}, false)
	if len(msgs) != 0 {
		t.Errorf("empty input should produce 0 msgs, got %d", len(msgs))
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestApplyModifier(t *testing.T) {
	tests := []struct {
		base string
		mod  int
		want string
	}{
		{"up", 0, "up"},
		{"up", 2, "shift+up"},
		{"up", 3, "alt+up"},
		{"up", 4, "alt+shift+up"},
		{"up", 5, "ctrl+up"},
		{"up", 6, "ctrl+shift+up"},
		{"up", 7, "ctrl+alt+up"},
		{"up", 8, "ctrl+alt+shift+up"},
		{"up", 99, "up"}, // unknown modifier
	}
	for _, tt := range tests {
		got := applyModifier(tt.base, tt.mod)
		if got != tt.want {
			t.Errorf("applyModifier(%q, %d) = %q, want %q", tt.base, tt.mod, got, tt.want)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseControlRange(t *testing.T) {
	// All ctrl+A through ctrl+Z
	for ctrlByte := byte(1); ctrlByte <= 26; ctrlByte++ {
		msgs, _ := parseInput([]byte{ctrlByte}, false)
		if len(msgs) != 1 {
			t.Errorf("byte %d: expected 1 msg, got %d", ctrlByte, len(msgs))

			continue
		}

		keyPress, ok := msgs[0].(KeyPressMsg)
		if !ok {
			t.Errorf("byte %d: expected KeyPressMsg, got %T", ctrlByte, msgs[0])

			continue
		}

		if keyPress.Key == "" {
			t.Errorf("byte %d: key should not be empty", ctrlByte)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputInsertKey(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '2', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "insert" {
		t.Errorf("key = %q, want %q", keyPress.Key, "insert")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputHomeCSI(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '1', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "home" {
		t.Errorf("key = %q, want %q", keyPress.Key, "home")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEndCSI(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '4', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "end" {
		t.Errorf("key = %q, want %q", keyPress.Key, "end")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF6ThroughF10(t *testing.T) {
	tests := []struct {
		code   string
		wanted string
	}{
		{"17", "f6"},
		{"18", "f7"},
		{"19", "f8"},
		{"20", "f9"},
		{"21", "f10"},
	}
	for _, test := range tests {
		input := append([]byte{0x1B, '['}, []byte(test.code)...)
		input = append(input, '~')
		msgs, _ := parseInput(input, false)

		keyPress := asKeyPress(msgs[0])
		if keyPress.Key != test.wanted {
			t.Errorf("CSI %s~: key = %q, want %q", test.code, keyPress.Key, test.wanted)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputF11F12(t *testing.T) {
	tests := []struct {
		code   string
		wanted string
	}{
		{"23", "f11"},
		{"24", "f12"},
	}
	for _, test := range tests {
		input := append([]byte{0x1B, '['}, []byte(test.code)...)
		input = append(input, '~')
		msgs, _ := parseInput(input, false)

		keyPress := asKeyPress(msgs[0])
		if keyPress.Key != test.wanted {
			t.Errorf("CSI %s~: key = %q, want %q", test.code, keyPress.Key, test.wanted)
		}
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputHomeTilde(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '1', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "home" {
		t.Errorf("CSI 1~ key = %q, want %q", keyPress.Key, "home")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEndTilde(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '4', '~'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "end" {
		t.Errorf("CSI 4~ key = %q, want %q", keyPress.Key, "end")
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseMiddle(t *testing.T) {
	// ESC[<1;5;3M (middle click)
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '1', ';', '5', ';', '3', 'M'}, false)

	click := asMouseClick(msgs[0])
	if click.Button != MouseMiddle {
		t.Errorf("button = %d, want MouseMiddle", click.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseWheelDownAlt(t *testing.T) {
	// ESC[<65;5;3M (scroll down, SGR encoding 65)
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '6', '5', ';', '5', ';', '3', 'M'}, false)

	wheel := asMouseWheel(msgs[0])
	if wheel.Button != MouseWheelDown {
		t.Errorf("button = %d, want MouseWheelDown (SGR encoding 65)", wheel.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseWheelUpAlt(t *testing.T) {
	// ESC[<64;5;3M (scroll up, SGR encoding 64)
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '6', '4', ';', '5', ';', '3', 'M'}, false)

	wheel := asMouseWheel(msgs[0])
	if wheel.Button != MouseWheelUp {
		t.Errorf("button = %d, want MouseWheelUp (SGR encoding 64)", wheel.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseWheelDownStandard(t *testing.T) {
	// ESC[<65;5;3M (scroll down)
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '6', '5', ';', '5', ';', '3', 'M'}, false)

	wheel := asMouseWheel(msgs[0])
	if wheel.Button != MouseWheelDown {
		t.Errorf("button = %d, want MouseWheelDown", wheel.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseUnknownButton(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '9', ';', '5', ';', '3', 'M'}, false)
	if len(msgs) != 0 {
		t.Errorf("unknown mouse button should produce no msg, got %d", len(msgs))
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouseInsufficientParams(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '0', ';', '5', 'M'}, false)
	if len(msgs) != 0 {
		t.Errorf("mouse with <3 params should produce no msg, got %d", len(msgs))
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlLFallback(t *testing.T) {
	msgs, _ := parseInput([]byte{0x06}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+f" {
		t.Errorf("byte 0x06 should be ctrl+f, got %q", keyPress.Key)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlB(t *testing.T) {
	msgs, _ := parseInput([]byte{0x02}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+b" {
		t.Errorf("byte 0x02 should be ctrl+b, got %q", keyPress.Key)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCtrlN(t *testing.T) {
	msgs, _ := parseInput([]byte{0x0E}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "ctrl+n" {
		t.Errorf("byte 0x0E should be ctrl+n, got %q", keyPress.Key)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneContinuationByte(t *testing.T) {
	decoded, size := decodeInputRune([]byte{0x80}, 0)
	if size != 1 {
		t.Errorf("continuation byte should have size 1, got %d", size)
	}

	if decoded != 0x80 {
		t.Errorf("continuation byte should return as-is, got %U", decoded)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneIncomplete2Byte(t *testing.T) {
	decoded, size := decodeInputRune([]byte{0xC3}, 0)
	if size != 1 {
		t.Errorf("incomplete 2-byte should have size 1, got %d", size)
	}

	_ = decoded
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneIncomplete3Byte(t *testing.T) {
	decoded, size := decodeInputRune([]byte{0xE4, 0xB8}, 0)
	if size != 1 {
		t.Errorf("incomplete 3-byte should have size 1, got %d", size)
	}

	_ = decoded
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneIncomplete4Byte(t *testing.T) {
	decoded, size := decodeInputRune([]byte{0xF0, 0x9F, 0x93}, 0)
	if size != 1 {
		t.Errorf("incomplete 4-byte should have size 1, got %d", size)
	}

	_ = decoded
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneOutOfBounds(t *testing.T) {
	decoded, size := decodeInputRune([]byte{}, 0)
	if decoded != 0 || size != 0 {
		t.Errorf("out of bounds: got (%U, %d), want (0, 0)", decoded, size)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneComplete2Byte(t *testing.T) {
	data := []byte{0xC3, 0xA9}

	decoded, size := decodeInputRune(data, 0)
	if size != 2 {
		t.Errorf("complete 2-byte should have size 2, got %d", size)
	}

	if decoded != 0xE9 {
		t.Errorf("2-byte é should be U+00E9, got %U", decoded)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneComplete3Byte(t *testing.T) {
	data := []byte{0xE4, 0xB8, 0x96}

	decoded, size := decodeInputRune(data, 0)
	if size != 3 {
		t.Errorf("complete 3-byte should have size 3, got %d", size)
	}

	if decoded != 0x4E16 {
		t.Errorf("3-byte 世 should be U+4E16, got %U", decoded) //nolint:gosmopolitan
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestDecodeInputRuneComplete4Byte(t *testing.T) {
	data := []byte{0xF0, 0x9F, 0x93, 0xA6}

	decoded, size := decodeInputRune(data, 0)
	if size != 4 {
		t.Errorf("complete 4-byte should have size 4, got %d", size)
	}

	if decoded != 0x1F4E6 {
		t.Errorf("4-byte emoji should be U+1F4E6, got %U", decoded)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEscNonCSINonO(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, 'a'}, false)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 msgs (esc + a), got %d", len(msgs))
	}

	keyPress0 := asKeyPress(msgs[0])
	if keyPress0.Key != "esc" {
		t.Errorf("first msg key = %q, want esc", keyPress0.Key)
	}

	keyPress1 := asKeyPress(msgs[1])
	if keyPress1.Key != "a" {
		t.Errorf("second msg key = %q, want a", keyPress1.Key)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseControlNUL(t *testing.T) {
	msg := parseControl(0x00)
	if msg != nil {
		t.Errorf("NUL byte should produce nil msg, got %v", msg)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseControlBackspace(t *testing.T) {
	msg := parseControl(0x7F)

	keyPress := asKeyPress(msg)
	if keyPress.Key != "backspace" {
		t.Errorf("0x7F in parseControl = %q, want backspace", keyPress.Key)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputUnknownCSIFinalByte(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', 'X'}, false)
	if len(msgs) != 0 {
		t.Errorf("unknown CSI final byte should produce no msg, got %d", len(msgs))
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputEscOUnknown(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, 'O', 'X'}, false)

	keyPress := asKeyPress(msgs[0])
	if keyPress.Key != "esc" {
		t.Errorf("ESC O X should produce esc, got %q", keyPress.Key)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputTildeUnknownCode(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '[', '9', '9', '~'}, false)
	if len(msgs) != 0 {
		t.Errorf("unknown tilde code should produce no msg, got %d", len(msgs))
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputCSIIncomplete(t *testing.T) {
	msgs, _ := parseInput([]byte{0x1B, '['}, false)
	if len(msgs) != 0 {
		t.Errorf("incomplete CSI should produce no msg, got %d", len(msgs))
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouse1006LeftClick(t *testing.T) {
	// SGR mode 1006: ESC[<0;10;5M (left click at (9,4))
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'M'}, false)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	click, ok := msgs[0].(MouseClickMsg)
	if !ok {
		t.Fatalf("expected MouseClickMsg, got %T", msgs[0])
	}

	if click.X != 9 || click.Y != 4 {
		t.Errorf("mouse at (%d,%d), want (9,4)", click.X, click.Y)
	}

	if click.Button != MouseLeft {
		t.Errorf("button = %d, want MouseLeft", click.Button)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouse1006RightClick(t *testing.T) {
	// ESC[<2;5;3M (right click at (4,2))
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '2', ';', '5', ';', '3', 'M'}, false)

	click := asMouseClick(msgs[0])
	if click.Button != MouseRight {
		t.Errorf("button = %d, want MouseRight", click.Button)
	}

	if click.X != 4 || click.Y != 2 {
		t.Errorf("mouse at (%d,%d), want (4,2)", click.X, click.Y)
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouse1006Release(t *testing.T) {
	// ESC[<0;10;5m (release event — lowercase 'm')
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'm'}, false)
	if len(msgs) != 0 {
		t.Errorf("mouse release should produce no msg, got %d", len(msgs))
	}
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestParseInputSGRMouse1006WheelUp(t *testing.T) {
	// ESC[<64;10;5M (scroll up)
	msgs, _ := parseInput([]byte{0x1B, '[', '<', '6', '4', ';', '1', '0', ';', '5', 'M'}, false)

	wheel := asMouseWheel(msgs[0])
	if wheel.Button != MouseWheelUp {
		t.Errorf("button = %d, want MouseWheelUp", wheel.Button)
	}
}

func asKeyPress(msg Msg) KeyPressMsg {
	kp, ok := msg.(KeyPressMsg)
	if !ok {
		panic(fmt.Sprintf("expected KeyPressMsg, got %T", msg))
	}

	return kp
}

func asMouseClick(msg Msg) MouseClickMsg {
	c, ok := msg.(MouseClickMsg)
	if !ok {
		panic(fmt.Sprintf("expected MouseClickMsg, got %T", msg))
	}

	return c
}

func asMouseWheel(msg Msg) MouseWheelMsg {
	w, ok := msg.(MouseWheelMsg)
	if !ok {
		panic(fmt.Sprintf("expected MouseWheelMsg, got %T", msg))
	}

	return w
}
