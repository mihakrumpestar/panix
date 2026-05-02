package render

import (
	"strings"
)

//nolint:cyclop,funlen,gocognit,mnd
func parseInput(data []byte) []Msg {
	var msgs []Msg

	pos := 0
	for pos < len(data) {
		b := data[pos]
		if b == '\x1b' {
			if pos+1 >= len(data) {
				msgs = append(msgs, KeyPressMsg{Key: "esc"})
				pos++

				continue
			}

			if data[pos+1] == '[' {
				msg, adv := parseCSI(data, pos+2)
				if msg != nil {
					msgs = append(msgs, msg)
				}

				pos = adv

				continue
			}

			if data[pos+1] == 'O' && pos+2 < len(data) {
				switch data[pos+2] {
				case 'P':
					msgs = append(msgs, KeyPressMsg{Key: "f1"})
				case 'Q':
					msgs = append(msgs, KeyPressMsg{Key: "f2"})
				case 'R':
					msgs = append(msgs, KeyPressMsg{Key: "f3"})
				case 'S':
					msgs = append(msgs, KeyPressMsg{Key: "f4"})
				default:
					msgs = append(msgs, KeyPressMsg{Key: "esc"})
				}

				pos += 3

				continue
			}

			msgs = append(msgs, KeyPressMsg{Key: "esc"})
			pos += 1

			continue
		}

		if b < 0x20 {
			msgs = append(msgs, parseControl(b))
			pos++

			continue
		}

		if b == 0x7F {
			msgs = append(msgs, KeyPressMsg{Key: "backspace"})
			pos++

			continue
		}

		r, size := decodeInputRune(data, pos)
		pos += size

		msgs = append(msgs, KeyPressMsg{Key: string(r)})
	}

	return msgs
}

//nolint:cyclop,funlen,mnd
func parseCSI(data []byte, pos int) (Msg, int) {
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
		return parseTilde(params, start), pos
	case 'M', 'm':
		return parseSGRMouse(params, final), pos
	case 'Z':
		return KeyPressMsg{Key: "backtab"}, pos
	}

	_ = start

	return nil, pos
}

//nolint:mnd
func parseTilde(params []int, start int) Msg {
	code := 0
	if len(params) > 0 {
		code = params[0]
	}

	mod := 0
	if len(params) > 1 {
		mod = params[1]
	}

	base := ""

	switch code {
	case 1:
		base = "home"
	case 2:
		base = "insert"
	case 3:
		base = "delete"
	case 4:
		base = "end"
	case 5:
		base = "pgup"
	case 6:
		base = "pgdown"
	case 11:
		base = "f1"
	case 12:
		base = "f2"
	case 13:
		base = "f3"
	case 14:
		base = "f4"
	case 15:
		base = "f5"
	case 17:
		base = "f6"
	case 18:
		base = "f7"
	case 19:
		base = "f8"
	case 20:
		base = "f9"
	case 21:
		base = "f10"
	case 23:
		base = "f11"
	case 24:
		base = "f12"
	default:
		return nil
	}

	return KeyPressMsg{Key: applyModifier(base, mod)}
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

//nolint:mnd
func parseSGRMouse(params []int, final byte) Msg {
	if len(params) < 3 {
		return nil
	}

	button := params[0]
	x := params[1] - 1
	y := params[2] - 1

	if x < 0 {
		x = 0
	}

	if y < 0 {
		y = 0
	}

	// SGR mouse: final 'M' = press, 'm' = release.
	// For release events, only report button releases for clicks (not motion).
	if final == 'm' {
		return nil
	}

	// Button field bits: bits 0-1 = button (0=left, 1=middle, 2=right),
	// bit 2 = shift, bit 3 = meta, bit 4 = ctrl, bit 5 = motion flag,
	// bit 6 = scroll wheel.
	switch button {
	case 0:
		return MouseClickMsg{X: x, Y: y, Button: MouseLeft}
	case 1:
		return MouseClickMsg{X: x, Y: y, Button: MouseMiddle}
	case 2:
		return MouseClickMsg{X: x, Y: y, Button: MouseRight}
	case 64:
		return MouseWheelMsg{X: x, Y: y, Button: MouseWheelUp}
	case 65:
		return MouseWheelMsg{X: x, Y: y, Button: MouseWheelDown}
	default:
		return nil
	}
}

//nolint:mnd
func parseControl(b byte) Msg {
	switch b {
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
		if b >= 1 && b <= 26 {
			return KeyPressMsg{Key: "ctrl+" + string('a'+b-1)}
		}

		return nil
	}
}

func decodeInputRune(data []byte, pos int) (rune, int) {
	if pos >= len(data) {
		return 0, 0
	}

	b := data[pos]
	if b < 0x80 {
		return rune(b), 1
	}

	if b < 0xC0 {
		return rune(b), 1
	}

	r := rune(b)
	size := 1

	switch {
	case r&0xE0 == 0xC0:
		if pos+1 < len(data) {
			r = rune(data[pos]&0x1F)<<6 | rune(data[pos+1]&0x3F)
			size = 2
		}
	case r&0xF0 == 0xE0:
		if pos+2 < len(data) {
			r = rune(data[pos]&0x0F)<<12 | rune(data[pos+1]&0x3F)<<6 | rune(data[pos+2]&0x3F)
			size = 3
		}
	case r&0xF8 == 0xF0:
		if pos+3 < len(data) {
			r = rune(data[pos]&0x07)<<18 | rune(data[pos+1]&0x3F)<<12 | rune(data[pos+2]&0x3F)<<6 | rune(data[pos+3]&0x3F)
			size = 4
		}
	}

	return r, size
}

type keyInfo struct {
	base string
	mod  string
}

var keyMap = map[string]keyInfo{
	"up":        {"up", ""},
	"down":      {"down", ""},
	"right":     {"right", ""},
	"left":      {"left", ""},
	"home":      {"home", ""},
	"end":       {"end", ""},
	"pgup":      {"pgup", ""},
	"pgdown":    {"pgdown", ""},
	"tab":       {"tab", ""},
	"backtab":   {"backtab", ""},
	"enter":     {"enter", ""},
	"backspace": {"backspace", ""},
	"esc":       {"esc", ""},
	"delete":    {"delete", ""},
	"insert":    {"insert", ""},
	"ctrl+c":    {"ctrl+c", ""},
	"ctrl+r":    {"ctrl+r", ""},
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
