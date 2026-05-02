package render

import (
	"testing"
)

func TestParseInputLetter(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte("a"))
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	kp, ok := msgs[0].(KeyPressMsg)
	if !ok {
		t.Fatalf("expected KeyPressMsg, got %T", msgs[0])
	}

	if kp.Key != "a" {
		t.Errorf("key = %q, want %q", kp.Key, "a")
	}
}

func TestParseInputEnter(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x0D})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "enter" {
		t.Errorf("key = %q, want %q", kp.Key, "enter")
	}
}

func TestParseInputTab(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x09})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "tab" {
		t.Errorf("key = %q, want %q", kp.Key, "tab")
	}
}

func TestParseInputBackspace(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x7F})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "backspace" {
		t.Errorf("key = %q, want %q", kp.Key, "backspace")
	}
}

func TestParseInputCtrlC(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x03})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+c" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+c")
	}
}

func TestParseInputCtrlD(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x04})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+d" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+d")
	}
}

func TestParseInputCtrlZ(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1A})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+z" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+z")
	}
}

func TestParseInputCtrlA(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x01})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+a" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+a")
	}
}

func TestParseInputCtrlE(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x05})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+e" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+e")
	}
}

func TestParseInputCtrlK(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x0B})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+k" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+k")
	}
}

func TestParseInputCtrlU(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x15})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+u" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+u")
	}
}

func TestParseInputCtrlW(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x17})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+w" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+w")
	}
}

func TestParseInputCtrlR(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x12})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+r" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+r")
	}
}

func TestParseInputNewline(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x0A})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "enter" {
		t.Errorf("key = %q, want %q (LF should be enter)", kp.Key, "enter")
	}
}

func TestParseInputEscapeAlone(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "esc" {
		t.Errorf("key = %q, want %q", kp.Key, "esc")
	}
}

func TestParseInputArrowUp(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'A'})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "up" {
		t.Errorf("key = %q, want %q", kp.Key, "up")
	}
}

func TestParseInputArrowDown(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'B'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "down" {
		t.Errorf("key = %q, want %q", kp.Key, "down")
	}
}

func TestParseInputArrowRight(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'C'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "right" {
		t.Errorf("key = %q, want %q", kp.Key, "right")
	}
}

func TestParseInputArrowLeft(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'D'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "left" {
		t.Errorf("key = %q, want %q", kp.Key, "left")
	}
}

func TestParseInputHome(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'H'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "home" {
		t.Errorf("key = %q, want %q", kp.Key, "home")
	}
}

func TestParseInputEnd(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'F'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "end" {
		t.Errorf("key = %q, want %q", kp.Key, "end")
	}
}

func TestParseInputDelete(t *testing.T) {
	t.Parallel()

	// CSI 3~
	msgs := parseInput([]byte{0x1B, '[', '3', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "delete" {
		t.Errorf("key = %q, want %q", kp.Key, "delete")
	}
}

func TestParseInputPageUp(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '5', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "pgup" {
		t.Errorf("key = %q, want %q", kp.Key, "pgup")
	}
}

func TestParseInputPageDown(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '6', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "pgdown" {
		t.Errorf("key = %q, want %q", kp.Key, "pgdown")
	}
}

func TestParseInputF1VT(t *testing.T) {
	t.Parallel()

	// ESC O P
	msgs := parseInput([]byte{0x1B, 'O', 'P'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "f1" {
		t.Errorf("key = %q, want %q", kp.Key, "f1")
	}
}

func TestParseInputF2VT(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, 'O', 'Q'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "f2" {
		t.Errorf("key = %q, want %q", kp.Key, "f2")
	}
}

func TestParseInputF3VT(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, 'O', 'R'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "f3" {
		t.Errorf("key = %q, want %q", kp.Key, "f3")
	}
}

func TestParseInputF4VT(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, 'O', 'S'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "f4" {
		t.Errorf("key = %q, want %q", kp.Key, "f4")
	}
}

func TestParseInputF1CSI(t *testing.T) {
	t.Parallel()

	// CSI 11~
	msgs := parseInput([]byte{0x1B, '[', '1', '1', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "f1" {
		t.Errorf("key = %q, want %q", kp.Key, "f1")
	}
}

func TestParseInputF5(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '1', '5', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "f5" {
		t.Errorf("key = %q, want %q", kp.Key, "f5")
	}
}

func TestParseInputF12(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '2', '4', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "f12" {
		t.Errorf("key = %q, want %q", kp.Key, "f12")
	}
}

func TestParseInputBacktab(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'Z'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "backtab" {
		t.Errorf("key = %q, want %q", kp.Key, "backtab")
	}
}

func TestParseInputSGRMouseLeftClick(t *testing.T) {
	t.Parallel()

	// SGR mode 1006: ESC[<0;10;5M (left click at (9,4))
	msgs := parseInput([]byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'M'})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	mc, ok := msgs[0].(MouseClickMsg)
	if !ok {
		t.Fatalf("expected MouseClickMsg, got %T", msgs[0])
	}

	if mc.X != 9 || mc.Y != 4 {
		t.Errorf("mouse at (%d,%d), want (9,4)", mc.X, mc.Y)
	}

	if mc.Button != MouseLeft {
		t.Errorf("button = %d, want MouseLeft", mc.Button)
	}
}

func TestParseInputSGRMouseRightClick(t *testing.T) {
	t.Parallel()

	// ESC[<2;5;3M (right click at (4,2))
	msgs := parseInput([]byte{0x1B, '[', '<', '2', ';', '5', ';', '3', 'M'})

	mc := msgs[0].(MouseClickMsg)
	if mc.Button != MouseRight {
		t.Errorf("button = %d, want MouseRight", mc.Button)
	}
}

func TestParseInputSGRMouseWheelUp(t *testing.T) {
	t.Parallel()

	// ESC[<64;10;5M (scroll up at (9,4))
	msgs := parseInput([]byte{0x1B, '[', '<', '6', '4', ';', '1', '0', ';', '5', 'M'})

	mw, ok := msgs[0].(MouseWheelMsg)
	if !ok {
		t.Fatalf("expected MouseWheelMsg, got %T", msgs[0])
	}

	if mw.Button != MouseWheelUp {
		t.Errorf("button = %d, want MouseWheelUp", mw.Button)
	}
}

func TestParseInputSGRMouseWheelDown(t *testing.T) {
	t.Parallel()

	// ESC[<65;10;5M (scroll down at (9,4))
	msgs := parseInput([]byte{0x1B, '[', '<', '6', '5', ';', '1', '0', ';', '5', 'M'})

	mw := msgs[0].(MouseWheelMsg)
	if mw.Button != MouseWheelDown {
		t.Errorf("button = %d, want MouseWheelDown", mw.Button)
	}
}

func TestParseInputMultipleKeys(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte("abc"))
	if len(msgs) != 3 {
		t.Fatalf("expected 3 msgs, got %d", len(msgs))
	}

	for i, want := range []string{"a", "b", "c"} {
		kp := msgs[i].(KeyPressMsg)
		if kp.Key != want {
			t.Errorf("msg[%d].Key = %q, want %q", i, kp.Key, want)
		}
	}
}

func TestParseInputUTF8(t *testing.T) {
	t.Parallel()

	// é = 0xc3 0xa9
	msgs := parseInput([]byte{0xc3, 0xa9})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "é" {
		t.Errorf("key = %q, want %q", kp.Key, "é")
	}
}

func TestParseInputShiftModifier(t *testing.T) {
	t.Parallel()

	// CSI 1;2A = shift+up
	msgs := parseInput([]byte{0x1B, '[', '1', ';', '2', 'A'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "shift+up" {
		t.Errorf("key = %q, want %q", kp.Key, "shift+up")
	}
}

func TestParseInputCtrlModifier(t *testing.T) {
	t.Parallel()

	// CSI 1;5A = ctrl+up
	msgs := parseInput([]byte{0x1B, '[', '1', ';', '5', 'A'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+up" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+up")
	}
}

func TestParseInputAltModifier(t *testing.T) {
	t.Parallel()

	// CSI 1;3A = alt+up
	msgs := parseInput([]byte{0x1B, '[', '1', ';', '3', 'A'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "alt+up" {
		t.Errorf("key = %q, want %q", kp.Key, "alt+up")
	}
}

func TestParseInputCtrlShiftModifier(t *testing.T) {
	t.Parallel()

	// CSI 1;6A = ctrl+shift+up
	msgs := parseInput([]byte{0x1B, '[', '1', ';', '6', 'A'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+shift+up" {
		t.Errorf("key = %q, want %q", kp.Key, "ctrl+shift+up")
	}
}

func TestParseInputIncompleteEscape(t *testing.T) {
	t.Parallel()

	// Lone ESC at end of buffer
	msgs := parseInput([]byte{0x1B})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "esc" {
		t.Errorf("key = %q, want %q", kp.Key, "esc")
	}
}

func TestParseInputEmptyInput(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{})
	if len(msgs) != 0 {
		t.Errorf("empty input should produce 0 msgs, got %d", len(msgs))
	}
}

func TestMatchKeyExact(t *testing.T) {
	t.Parallel()

	if !MatchKey("enter", "enter") {
		t.Error("MatchKey should match exact keys")
	}

	if MatchKey("enter", "tab") {
		t.Error("MatchKey should not match different keys")
	}
}

func TestMatchKeyCtrl(t *testing.T) {
	t.Parallel()

	if !MatchKey("ctrl+c", "ctrl+c") {
		t.Error("MatchKey should match ctrl+key")
	}

	if MatchKey("ctrl+c", "ctrl+d") {
		t.Error("MatchKey should not match different ctrl combos")
	}
}

func TestMatchKeyCtrlNonCtrl(t *testing.T) {
	t.Parallel()

	if MatchKey("ctrl+c", "c") {
		t.Error("MatchKey should not match ctrl+key against plain key")
	}

	if MatchKey("a", "ctrl+a") {
		t.Error("MatchKey should not match plain key against ctrl+key")
	}
}

func TestApplyModifier(t *testing.T) {
	t.Parallel()

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

func TestParseControlRange(t *testing.T) {
	t.Parallel()

	// All ctrl+A through ctrl+Z
	for b := byte(1); b <= 26; b++ {
		msgs := parseInput([]byte{b})
		if len(msgs) != 1 {
			t.Errorf("byte %d: expected 1 msg, got %d", b, len(msgs))

			continue
		}

		kp, ok := msgs[0].(KeyPressMsg)
		if !ok {
			t.Errorf("byte %d: expected KeyPressMsg, got %T", b, msgs[0])

			continue
		}

		if kp.Key == "" {
			t.Errorf("byte %d: key should not be empty", b)
		}
	}
}

func TestParseInputInsertKey(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '2', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "insert" {
		t.Errorf("key = %q, want %q", kp.Key, "insert")
	}
}

func TestParseInputHomeCSI(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '1', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "home" {
		t.Errorf("key = %q, want %q", kp.Key, "home")
	}
}

func TestParseInputEndCSI(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '4', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "end" {
		t.Errorf("key = %q, want %q", kp.Key, "end")
	}
}

func TestParseInputF6ThroughF10(t *testing.T) {
	t.Parallel()

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
	for _, tt := range tests {
		input := append([]byte{0x1B, '['}, []byte(tt.code)...)
		input = append(input, '~')
		msgs := parseInput(input)

		kp := msgs[0].(KeyPressMsg)
		if kp.Key != tt.wanted {
			t.Errorf("CSI %s~: key = %q, want %q", tt.code, kp.Key, tt.wanted)
		}
	}
}

func TestParseInputF11F12(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code   string
		wanted string
	}{
		{"23", "f11"},
		{"24", "f12"},
	}
	for _, tt := range tests {
		input := append([]byte{0x1B, '['}, []byte(tt.code)...)
		input = append(input, '~')
		msgs := parseInput(input)

		kp := msgs[0].(KeyPressMsg)
		if kp.Key != tt.wanted {
			t.Errorf("CSI %s~: key = %q, want %q", tt.code, kp.Key, tt.wanted)
		}
	}
}

func TestParseInputHomeTilde(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '1', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "home" {
		t.Errorf("CSI 1~ key = %q, want %q", kp.Key, "home")
	}
}

func TestParseInputEndTilde(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '4', '~'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "end" {
		t.Errorf("CSI 4~ key = %q, want %q", kp.Key, "end")
	}
}

func TestParseInputSGRMouseMiddle(t *testing.T) {
	t.Parallel()

	// ESC[<1;5;3M (middle click)
	msgs := parseInput([]byte{0x1B, '[', '<', '1', ';', '5', ';', '3', 'M'})

	mc := msgs[0].(MouseClickMsg)
	if mc.Button != MouseMiddle {
		t.Errorf("button = %d, want MouseMiddle", mc.Button)
	}
}

func TestParseInputSGRMouseWheelDownAlt(t *testing.T) {
	t.Parallel()

	// ESC[<65;5;3M (scroll down, SGR encoding 65)
	msgs := parseInput([]byte{0x1B, '[', '<', '6', '5', ';', '5', ';', '3', 'M'})

	mw := msgs[0].(MouseWheelMsg)
	if mw.Button != MouseWheelDown {
		t.Errorf("button = %d, want MouseWheelDown (SGR encoding 65)", mw.Button)
	}
}

func TestParseInputSGRMouseWheelUpAlt(t *testing.T) {
	t.Parallel()

	// ESC[<64;5;3M (scroll up, SGR encoding 64)
	msgs := parseInput([]byte{0x1B, '[', '<', '6', '4', ';', '5', ';', '3', 'M'})

	mw := msgs[0].(MouseWheelMsg)
	if mw.Button != MouseWheelUp {
		t.Errorf("button = %d, want MouseWheelUp (SGR encoding 64)", mw.Button)
	}
}

func TestParseInputSGRMouseWheelDownStandard(t *testing.T) {
	t.Parallel()

	// ESC[<65;5;3M (scroll down)
	msgs := parseInput([]byte{0x1B, '[', '<', '6', '5', ';', '5', ';', '3', 'M'})

	mw := msgs[0].(MouseWheelMsg)
	if mw.Button != MouseWheelDown {
		t.Errorf("button = %d, want MouseWheelDown", mw.Button)
	}
}

func TestParseInputSGRMouseUnknownButton(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '9', ';', '5', ';', '3', 'M'})
	if len(msgs) != 0 {
		t.Errorf("unknown mouse button should produce no msg, got %d", len(msgs))
	}
}

func TestParseInputSGRMouseInsufficientParams(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '0', ';', '5', 'M'})
	if len(msgs) != 0 {
		t.Errorf("mouse with <3 params should produce no msg, got %d", len(msgs))
	}
}

func TestParseInputCtrlLFallback(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x06})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+f" {
		t.Errorf("byte 0x06 should be ctrl+f, got %q", kp.Key)
	}
}

func TestParseInputCtrlB(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x02})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+b" {
		t.Errorf("byte 0x02 should be ctrl+b, got %q", kp.Key)
	}
}

func TestParseInputCtrlN(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x0E})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "ctrl+n" {
		t.Errorf("byte 0x0E should be ctrl+n, got %q", kp.Key)
	}
}

func TestDecodeInputRuneContinuationByte(t *testing.T) {
	t.Parallel()

	r, size := decodeInputRune([]byte{0x80}, 0)
	if size != 1 {
		t.Errorf("continuation byte should have size 1, got %d", size)
	}

	if r != 0x80 {
		t.Errorf("continuation byte should return as-is, got %U", r)
	}
}

func TestDecodeInputRuneIncomplete2Byte(t *testing.T) {
	t.Parallel()

	r, size := decodeInputRune([]byte{0xC3}, 0)
	if size != 1 {
		t.Errorf("incomplete 2-byte should have size 1, got %d", size)
	}

	_ = r
}

func TestDecodeInputRuneIncomplete3Byte(t *testing.T) {
	t.Parallel()

	r, size := decodeInputRune([]byte{0xE4, 0xB8}, 0)
	if size != 1 {
		t.Errorf("incomplete 3-byte should have size 1, got %d", size)
	}

	_ = r
}

func TestDecodeInputRuneIncomplete4Byte(t *testing.T) {
	t.Parallel()

	r, size := decodeInputRune([]byte{0xF0, 0x9F, 0x93}, 0)
	if size != 1 {
		t.Errorf("incomplete 4-byte should have size 1, got %d", size)
	}

	_ = r
}

func TestDecodeInputRuneOutOfBounds(t *testing.T) {
	t.Parallel()

	r, size := decodeInputRune([]byte{}, 0)
	if r != 0 || size != 0 {
		t.Errorf("out of bounds: got (%U, %d), want (0, 0)", r, size)
	}
}

func TestDecodeInputRuneComplete2Byte(t *testing.T) {
	t.Parallel()

	data := []byte{0xC3, 0xA9}

	r, size := decodeInputRune(data, 0)
	if size != 2 {
		t.Errorf("complete 2-byte should have size 2, got %d", size)
	}

	if r != 0xE9 {
		t.Errorf("2-byte é should be U+00E9, got %U", r)
	}
}

func TestDecodeInputRuneComplete3Byte(t *testing.T) {
	t.Parallel()

	data := []byte{0xE4, 0xB8, 0x96}

	r, size := decodeInputRune(data, 0)
	if size != 3 {
		t.Errorf("complete 3-byte should have size 3, got %d", size)
	}

	if r != 0x4E16 {
		t.Errorf("3-byte 世 should be U+4E16, got %U", r)
	}
}

func TestDecodeInputRuneComplete4Byte(t *testing.T) {
	t.Parallel()

	data := []byte{0xF0, 0x9F, 0x93, 0xA6}

	r, size := decodeInputRune(data, 0)
	if size != 4 {
		t.Errorf("complete 4-byte should have size 4, got %d", size)
	}

	if r != 0x1F4E6 {
		t.Errorf("4-byte emoji should be U+1F4E6, got %U", r)
	}
}

func TestParseInputEscNonCSINonO(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, 'a'})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 msgs (esc + a), got %d", len(msgs))
	}

	kp0 := msgs[0].(KeyPressMsg)
	if kp0.Key != "esc" {
		t.Errorf("first msg key = %q, want esc", kp0.Key)
	}

	kp1 := msgs[1].(KeyPressMsg)
	if kp1.Key != "a" {
		t.Errorf("second msg key = %q, want a", kp1.Key)
	}
}

func TestParseControlNUL(t *testing.T) {
	t.Parallel()

	msg := parseControl(0x00)
	if msg != nil {
		t.Errorf("NUL byte should produce nil msg, got %v", msg)
	}
}

func TestParseControlBackspace(t *testing.T) {
	t.Parallel()

	msg := parseControl(0x7F)

	kp := msg.(KeyPressMsg)
	if kp.Key != "backspace" {
		t.Errorf("0x7F in parseControl = %q, want backspace", kp.Key)
	}
}

func TestParseInputUnknownCSIFinalByte(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', 'X'})
	if len(msgs) != 0 {
		t.Errorf("unknown CSI final byte should produce no msg, got %d", len(msgs))
	}
}

func TestParseInputEscOUnknown(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, 'O', 'X'})

	kp := msgs[0].(KeyPressMsg)
	if kp.Key != "esc" {
		t.Errorf("ESC O X should produce esc, got %q", kp.Key)
	}
}

func TestParseInputTildeUnknownCode(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '[', '9', '9', '~'})
	if len(msgs) != 0 {
		t.Errorf("unknown tilde code should produce no msg, got %d", len(msgs))
	}
}

func TestParseInputCSIIncomplete(t *testing.T) {
	t.Parallel()

	msgs := parseInput([]byte{0x1B, '['})
	if len(msgs) != 0 {
		t.Errorf("incomplete CSI should produce no msg, got %d", len(msgs))
	}
}

func TestParseInputSGRMouse1006LeftClick(t *testing.T) {
	t.Parallel()

	// SGR mode 1006: ESC[<0;10;5M (left click at (9,4))
	msgs := parseInput([]byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'M'})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}

	mc, ok := msgs[0].(MouseClickMsg)
	if !ok {
		t.Fatalf("expected MouseClickMsg, got %T", msgs[0])
	}

	if mc.X != 9 || mc.Y != 4 {
		t.Errorf("mouse at (%d,%d), want (9,4)", mc.X, mc.Y)
	}

	if mc.Button != MouseLeft {
		t.Errorf("button = %d, want MouseLeft", mc.Button)
	}
}

func TestParseInputSGRMouse1006RightClick(t *testing.T) {
	t.Parallel()

	// ESC[<2;5;3M (right click at (4,2))
	msgs := parseInput([]byte{0x1B, '[', '<', '2', ';', '5', ';', '3', 'M'})

	mc := msgs[0].(MouseClickMsg)
	if mc.Button != MouseRight {
		t.Errorf("button = %d, want MouseRight", mc.Button)
	}

	if mc.X != 4 || mc.Y != 2 {
		t.Errorf("mouse at (%d,%d), want (4,2)", mc.X, mc.Y)
	}
}

func TestParseInputSGRMouse1006Release(t *testing.T) {
	t.Parallel()

	// ESC[<0;10;5m (release event — lowercase 'm')
	msgs := parseInput([]byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'm'})
	if len(msgs) != 0 {
		t.Errorf("mouse release should produce no msg, got %d", len(msgs))
	}
}

func TestParseInputSGRMouse1006WheelUp(t *testing.T) {
	t.Parallel()

	// ESC[<64;10;5M (scroll up)
	msgs := parseInput([]byte{0x1B, '[', '<', '6', '4', ';', '1', '0', ';', '5', 'M'})

	mw := msgs[0].(MouseWheelMsg)
	if mw.Button != MouseWheelUp {
		t.Errorf("button = %d, want MouseWheelUp", mw.Button)
	}
}
