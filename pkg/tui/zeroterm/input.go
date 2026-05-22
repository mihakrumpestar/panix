// Derived from charm.land/bubbletea/v2. See pkg/tui/LICENSE.charmbracelet.

package zeroterm

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

var ss3Key = map[byte]string{
	'P': "f1", 'Q': "f2", 'R': "f3", 'S': "f4",
}

func parseEscape(data []byte, pos int, canHaveMoreData bool) ([]Msg, int) {
	if pos+1 >= len(data) {
		if canHaveMoreData {
			return nil, pos
		}

		return []Msg{KeyPressMsg{Key: "esc"}}, pos + 1
	}

	if data[pos+1] == '[' {
		msg, adv := parseCSI(data, pos+2, canHaveMoreData)

		var result []Msg
		if msg != nil {
			result = []Msg{msg}
		}

		return result, adv
	}

	if data[pos+1] == 'O' && pos+2 < len(data) {
		key, ok := ss3Key[data[pos+2]]
		if !ok {
			key = "esc"
		}

		return []Msg{KeyPressMsg{Key: key}}, pos + 3 //nolint:mnd
	}

	return []Msg{KeyPressMsg{Key: "esc"}}, pos + 1
}

//nolint:cyclop,funlen
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
		currentParam = currentParam*10 + int(data[pos]-'0') //nolint:mnd
		hasParam = true
		pos++
	}

	params = append(params, currentParam)

	for pos < len(data) && data[pos] == ';' {
		pos++
		currentParam = 0
		hasParam = false

		for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
			currentParam = currentParam*10 + int(data[pos]-'0') //nolint:mnd
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

var tildeCodeMap = map[int]string{
	1: "home", 2: "insert", 3: "delete", 4: "end",
	5: "pgup", 6: "pgdown",
	11: "f1", 12: "f2", 13: "f3", 14: "f4", 15: "f5",
	17: "f6", 18: "f7", 19: "f8", 20: "f9", 21: "f10",
	23: "f11", 24: "f12",
}

func tildeCodeToKey(code int) string {
	return tildeCodeMap[code]
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
func parseX10Wheel(btn, mouseX, mouseY int) Msg {
	switch btn {
	case 0:
		return MouseWheelMsg{X: mouseX, Y: mouseY, Button: MouseWheelUp}
	case 1:
		return MouseWheelMsg{X: mouseX, Y: mouseY, Button: MouseWheelDown}
	default:
		return nil
	}
}

func parseX10Click(btn, mouseX, mouseY int) Msg {
	switch btn {
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

const (
	bitWheel  = 0b0100_0000
	bitAdd    = 0b1000_0000
	bitMotion = 0b0010_0000
	bitsMask  = 0b0000_0011
)

//nolint:mnd
func parseX10Mouse(data []byte, cbPos int) Msg {
	clickBtn := int(data[cbPos]) - 32
	mouseX := max(0, int(data[cbPos+1])-33)
	mouseY := max(0, int(data[cbPos+2])-33)

	if clickBtn&bitWheel != 0 {
		return parseX10Wheel(clickBtn&bitsMask, mouseX, mouseY)
	}

	if clickBtn&(bitAdd|bitMotion) != 0 {
		return nil
	}

	return parseX10Click(clickBtn&bitsMask, mouseX, mouseY)
}

const sgrMouseMinParams = 3

func parseSGRMouse(params []int, final byte) Msg {
	if len(params) < sgrMouseMinParams {
		return nil
	}

	return parseSGRMouseParams(params, final)
}

func parseSGRMouseParams(params []int, final byte) Msg {
	if final == 'm' {
		return nil
	}

	button := params[0]
	mouseX := max(0, params[1]-1)
	mouseY := max(0, params[2]-1)

	if button&bitWheel != 0 {
		return parseX10Wheel(button&bitsMask, mouseX, mouseY)
	}

	if button&(bitAdd|bitMotion) != 0 {
		return nil
	}

	return parseX10Click(button&bitsMask, mouseX, mouseY)
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

var ctrlKeyMap = map[byte]string{
	0x0D: "enter", 0x09: "tab", 0x7F: "backspace", 0x0A: "enter",
	0x03: "ctrl+c", 0x04: "ctrl+d", 0x1A: "ctrl+z",
	0x15: "ctrl+u", 0x17: "ctrl+w", 0x12: "ctrl+r",
	0x01: "ctrl+a", 0x05: "ctrl+e", 0x0B: "ctrl+k",
}

func parseControl(byteVal byte) Msg {
	if key, ok := ctrlKeyMap[byteVal]; ok {
		return KeyPressMsg{Key: key}
	}

	if byteVal >= 1 && byteVal <= 26 {
		return KeyPressMsg{Key: "ctrl+" + string('a'+byteVal-1)}
	}

	return nil
}
