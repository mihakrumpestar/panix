package zeroterm

import (
	"strings"
)

const x10MouseLen = 6

// parseInput parses raw terminal input bytes into Messages.
// canHaveMoreData indicates that the read buffer was full and more data
// may be available — incomplete sequences should not be flushed.
//
//nolint:mnd
func parseInput(data []byte, canHaveMoreData bool) ([]Msg, int) {
	var msgs []Msg

	pos := 0
	for pos < len(data) {
		// Detect X10 mouse events first (before CSI parsing).
		// X10 format: ESC[M Cb Cx Cy — the 3 bytes after M are raw
		// (offset by 32), not valid CSI parameters, so they MUST be
		// consumed here before the CSI parser misinterprets them.
		//nolint:gosec // G602: safe — bounds check on left ensures pos+N < len(data)
		if pos+x10MouseLen <= len(data) && data[pos] == '\x1b' && data[pos+1] == '[' && data[pos+2] == 'M' {
			msgs = append(msgs, parseX10Mouse(data, pos+3))
			pos += x10MouseLen

			continue
		}

		byteVal := data[pos]
		if byteVal == '\x1b' {
			extraMsgs, adv := parseEscape(data, pos, canHaveMoreData)
			if adv == pos {
				return msgs, pos
			}

			msgs = append(msgs, extraMsgs...)
			pos = adv

			continue
		}

		if byteVal < 0x20 {
			msgs = append(msgs, parseControl(byteVal))
			pos++

			continue
		}

		if byteVal == 0x7F {
			msgs = append(msgs, KeyPressMsg{Key: "backspace"})
			pos++

			continue
		}

		decodedRune, size := decodeInputRune(data, pos)
		pos += size

		msgs = append(msgs, KeyPressMsg{Key: string(decodedRune)})
	}

	return msgs, pos
}

// parseEscape handles an ESC byte at data[pos]. It returns messages and
// the new position. If the sequence is incomplete and more data may arrive,
// it returns pos unchanged so the caller can return and wait.
//
//nolint:cyclop
func parseEscape(data []byte, pos int, canHaveMoreData bool) ([]Msg, int) {
	if pos+1 >= len(data) {
		if canHaveMoreData {
			return nil, pos
		}

		return []Msg{KeyPressMsg{Key: "esc"}}, pos + 1
	}

	if data[pos+1] == '[' {
		msg, adv := parseCSI(data, pos+2, canHaveMoreData) //nolint:mnd

		var result []Msg
		if msg != nil {
			result = []Msg{msg}
		}

		return result, adv
	}

	if data[pos+1] == 'O' && pos+2 < len(data) {
		var msg Msg

		switch data[pos+2] {
		case 'P':
			msg = KeyPressMsg{Key: "f1"}
		case 'Q':
			msg = KeyPressMsg{Key: "f2"}
		case 'R':
			msg = KeyPressMsg{Key: "f3"}
		case 'S':
			msg = KeyPressMsg{Key: "f4"}
		default:
			msg = KeyPressMsg{Key: "esc"}
		}

		return []Msg{msg}, pos + 3 //nolint:mnd
	}

	return []Msg{KeyPressMsg{Key: "esc"}}, pos + 1
}

//nolint:cyclop,funlen,mnd
func parseCSI(data []byte, pos int, canHaveMoreData bool) (Msg, int) {
	start := pos

	var params []int

	currentParam := 0
	hasParam := false

	// Handle intermediate byte '<' used by SGR mouse mode (1006).
	// Format: ESC[<Cb;Cx;CyM or ESC[<Cb;Cx;Cym
	isSGRMouse := false
	if pos < len(data) && data[pos] == '<' {
		isSGRMouse = true
		pos++
	}

	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		currentParam = currentParam*10 + int(data[pos]-'0')
		hasParam = true
		pos++
	}

	params = append(params, currentParam)

	for pos < len(data) && data[pos] == ';' {
		pos++
		currentParam = 0
		hasParam = false

		for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
			currentParam = currentParam*10 + int(data[pos]-'0')
			hasParam = true
			pos++
		}

		params = append(params, currentParam)
	}

	if hasParam {
		params[len(params)-1] = currentParam
	}

	if pos >= len(data) {
		if canHaveMoreData {
			return nil, start
		}

		return nil, pos
	}

	final := data[pos]
	pos++

	if isSGRMouse {
		if final == 'M' || final == 'm' {
			return parseSGRMouse(params, final), pos
		}

		return nil, pos
	}

	switch final {
	case 'A':
		return KeyPressMsg{Key: keyWithMod("up", params)}, pos
	case 'B':
		return KeyPressMsg{Key: keyWithMod("down", params)}, pos
	case 'C':
		return KeyPressMsg{Key: keyWithMod("right", params)}, pos
	case 'D':
		return KeyPressMsg{Key: keyWithMod("left", params)}, pos
	case 'H':
		return KeyPressMsg{Key: keyWithMod("home", params)}, pos
	case 'F':
		return KeyPressMsg{Key: keyWithMod("end", params)}, pos
	case '~':
		return parseTilde(params), pos
	case 'Z':
		return KeyPressMsg{Key: "backtab"}, pos
	}

	_ = start

	return nil, pos
}

func parseTilde(params []int) Msg {
	code := 0
	if len(params) > 0 {
		code = params[0]
	}

	mod := 0
	if len(params) > 1 {
		mod = params[1]
	}

	base := tildeCodeToKey(code)
	if base == "" {
		return nil
	}

	return KeyPressMsg{Key: applyModifier(base, mod)}
}

// tildeCodeToKey maps a CSI tilde code to its key name.
//
//nolint:cyclop,mnd
func tildeCodeToKey(code int) string {
	switch code {
	case 1:
		return "home"
	case 2:
		return "insert"
	case 3:
		return "delete"
	case 4:
		return "end"
	case 5:
		return "pgup"
	case 6:
		return "pgdown"
	case 11:
		return "f1"
	case 12:
		return "f2"
	case 13:
		return "f3"
	case 14:
		return "f4"
	case 15:
		return "f5"
	case 17:
		return "f6"
	case 18:
		return "f7"
	case 19:
		return "f8"
	case 20:
		return "f9"
	case 21:
		return "f10"
	case 23:
		return "f11"
	case 24:
		return "f12"
	default:
		return ""
	}
}

func keyWithMod(base string, params []int) string {
	mod := 0
	if len(params) > 1 {
		mod = params[1]
	}

	return applyModifier(base, mod)
}

//nolint:mnd
func applyModifier(base string, mod int) string {
	switch mod {
	case 2:
		return "shift+" + base
	case 3:
		return "alt+" + base
	case 4:
		return "alt+shift+" + base
	case 5:
		return "ctrl+" + base
	case 6:
		return "ctrl+shift+" + base
	case 7:
		return "ctrl+alt+" + base
	case 8:
		return "ctrl+alt+shift+" + base
	default:
		return base
	}
}

// parseX10Mouse parses an X10 mouse event.
// Format: ESC[M Cb Cx Cy — Cb, Cx, Cy are single bytes offset by 32.
// cbPos points to the first byte after 'M' in data.
//
// Button field bits (after subtracting 32):
//   - bits 0-1: button (0=left, 1=middle, 2=right, 3=release)
//   - bit 2: shift
//   - bit 3: meta (alt)
//   - bit 4: ctrl
//   - bit 5: motion
//   - bit 6: scroll wheel
//   - bit 7: additional buttons 8-11
//
//nolint:cyclop,mnd
func parseX10Mouse(data []byte, cbPos int) Msg {
	clickBtn := int(data[cbPos]) - 32
	cx := int(data[cbPos+1]) - 32
	cy := int(data[cbPos+2]) - 32

	mouseX := cx - 1
	mouseY := cy - 1

	if mouseX < 0 {
		mouseX = 0
	}

	if mouseY < 0 {
		mouseY = 0
	}

	const (
		bitWheel  = 0b0100_0000
		bitAdd    = 0b1000_0000
		bitsMask  = 0b0000_0011
		bitMotion = 0b0010_0000
	)

	if clickBtn&bitWheel != 0 {
		switch clickBtn & bitsMask {
		case 0:
			return MouseWheelMsg{X: mouseX, Y: mouseY, Button: MouseWheelUp}
		case 1:
			return MouseWheelMsg{X: mouseX, Y: mouseY, Button: MouseWheelDown}
		}

		return nil
	}

	if clickBtn&bitAdd != 0 {
		return nil
	}

	// Motion events (button held + drag) — not needed for TUI
	if clickBtn&bitMotion != 0 {
		return nil
	}

	switch clickBtn & bitsMask {
	case 0:
		return MouseClickMsg{X: mouseX, Y: mouseY, Button: MouseLeft}
	case 1:
		return MouseClickMsg{X: mouseX, Y: mouseY, Button: MouseMiddle}
	case 2:
		return MouseClickMsg{X: mouseX, Y: mouseY, Button: MouseRight}
	case 3:
		// Button release — not reported as click
		return nil
	default:
		return nil
	}
}

//nolint:cyclop,mnd
func parseSGRMouse(params []int, final byte) Msg {
	if len(params) < 3 {
		return nil
	}

	button := params[0]
	mouseX := params[1] - 1
	mouseY := params[2] - 1

	if mouseX < 0 {
		mouseX = 0
	}

	if mouseY < 0 {
		mouseY = 0
	}

	// SGR mouse: final 'M' = press, 'm' = release.
	// For release events, only report button releases for clicks (not motion).
	if final == 'm' {
		return nil
	}

	const (
		bitWheel  = 0b0100_0000
		bitAdd    = 0b1000_0000
		bitsMask  = 0b0000_0011
		bitMotion = 0b0010_0000
	)

	if button&bitWheel != 0 {
		switch button & bitsMask {
		case 0:
			return MouseWheelMsg{X: mouseX, Y: mouseY, Button: MouseWheelUp}
		case 1:
			return MouseWheelMsg{X: mouseX, Y: mouseY, Button: MouseWheelDown}
		}

		return nil
	}

	if button&bitAdd != 0 {
		return nil
	}

	if button&bitMotion != 0 {
		return nil
	}

	switch button & bitsMask {
	case 0:
		return MouseClickMsg{X: mouseX, Y: mouseY, Button: MouseLeft}
	case 1:
		return MouseClickMsg{X: mouseX, Y: mouseY, Button: MouseMiddle}
	case 2:
		return MouseClickMsg{X: mouseX, Y: mouseY, Button: MouseRight}
	default:
		return nil
	}
}

//nolint:cyclop,mnd
func parseControl(byteVal byte) Msg {
	switch byteVal {
	case 0x0D:
		return KeyPressMsg{Key: "enter"}
	case 0x09:
		return KeyPressMsg{Key: "tab"}
	case 0x7F:
		return KeyPressMsg{Key: "backspace"}
	case 0x03:
		return KeyPressMsg{Key: "ctrl+c"}
	case 0x04:
		return KeyPressMsg{Key: "ctrl+d"}
	case 0x1A:
		return KeyPressMsg{Key: "ctrl+z"}
	case 0x15:
		return KeyPressMsg{Key: "ctrl+u"}
	case 0x17:
		return KeyPressMsg{Key: "ctrl+w"}
	case 0x12:
		return KeyPressMsg{Key: "ctrl+r"}
	case 0x01:
		return KeyPressMsg{Key: "ctrl+a"}
	case 0x05:
		return KeyPressMsg{Key: "ctrl+e"}
	case 0x0B:
		return KeyPressMsg{Key: "ctrl+k"}
	case 0x0A:
		return KeyPressMsg{Key: "enter"}
	default:
		if byteVal >= 1 && byteVal <= 26 {
			return KeyPressMsg{Key: "ctrl+" + string('a'+byteVal-1)}
		}

		return nil
	}
}

//nolint:mnd
func decodeInputRune(data []byte, pos int) (rune, int) {
	if pos >= len(data) {
		return 0, 0
	}

	firstByte := data[pos]
	if firstByte < 0x80 {
		return rune(firstByte), 1
	}

	if firstByte < 0xC0 {
		return rune(firstByte), 1
	}

	decoded := rune(firstByte)
	size := 1

	switch {
	case decoded&0xE0 == 0xC0:
		if pos+1 < len(data) {
			decoded = rune(data[pos]&0x1F)<<6 | rune(data[pos+1]&0x3F)
			size = 2
		}
	case decoded&0xF0 == 0xE0:
		if pos+2 < len(data) {
			decoded = rune(data[pos]&0x0F)<<12 | rune(data[pos+1]&0x3F)<<6 | rune(data[pos+2]&0x3F)
			size = 3
		}
	case decoded&0xF8 == 0xF0:
		if pos+3 < len(data) {
			decoded = rune(data[pos]&0x07)<<18 | rune(data[pos+1]&0x3F)<<12 | rune(data[pos+2]&0x3F)<<6 | rune(data[pos+3]&0x3F)
			size = 4
		}
	}

	return decoded, size
}

func MatchKey(key string, target string) bool {
	if key == target {
		return true
	}

	if strings.HasPrefix(key, "ctrl+") && strings.HasPrefix(target, "ctrl+") {
		return strings.TrimPrefix(key, "ctrl+") == strings.TrimPrefix(target, "ctrl+")
	}

	return false
}
