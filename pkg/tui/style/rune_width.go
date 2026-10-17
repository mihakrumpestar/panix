package style

import "unicode"

// runeWidth returns the terminal cell width of a single rune.
//
// Grapheme segmentation (ZWJ, skin tones) is unnecessary: the TUI renders
// terminal content and a fixed emoji set. Terminal-measured overrides win first;
// controls, combining marks and zero-width controls are 0; East Asian
// wide/fullwidth and Emoji_Presentation=Yes runes are 2, mirroring
// go-runewidth's doublewidth table; everything else is 1. Combining marks are
// checked by Unicode category before the wide ranges, so marks such as U+3099
// that live inside a CJK block still measure 0. The static table stays a pure
// fallback; overrides are installed separately.
func runeWidth(target rune) int {
	if overrides := measuredGlyphWidths.Load(); overrides != nil {
		if width, ok := overrides.runes[target]; ok {
			return width
		}
	}

	if unicode.Is(unicode.Mn, target) || unicode.Is(unicode.Me, target) {
		return 0
	}

	for _, entry := range widthRanges {
		if target >= entry.lo && target <= entry.hi {
			return entry.width
		}
	}

	return 1
}

type widthRange struct {
	lo, hi rune
	width  int
}

// widthRanges lists the zero-width (width 0) and wide (width 2) rune ranges,
// sorted by lo. Runes outside these ranges have width 1.
var widthRanges = []widthRange{
	// Zero-width: C0 controls, DEL, combining marks, variation selectors,
	// and zero-width controls.
	{0x0000, 0x001F, 0}, // C0 control characters
	{0x007F, 0x007F, 0}, // DEL
	{0x0300, 0x036F, 0}, // Combining Diacritical Marks
	{0x1AB0, 0x1AFF, 0}, // Combining Diacritical Marks Extended
	{0x1DC0, 0x1DFF, 0}, // Combining Diacritical Marks Supplement
	{0x200B, 0x200F, 0}, // ZWSP, ZWNJ, ZWJ, LRM, RLM
	{0x2028, 0x202E, 0}, // line sep, paragraph sep, bidi controls
	{0x2060, 0x2064, 0}, // word joiner, invisible operators
	{0x20D0, 0x20FF, 0}, // Combining Diacritical Marks for Symbols
	{0xFE00, 0xFE0F, 0}, // Variation Selectors
	{0xFE20, 0xFE2F, 0}, // Combining Half Marks

	// Wide: East Asian wide/fullwidth and emoji.
	{0x1100, 0x115F, 2},   // Hangul Jamo
	{0x231A, 0x231B, 2},   // watch, hourglass
	{0x2329, 0x232A, 2},   // angle brackets
	{0x23E9, 0x23EC, 2},   // media control symbols
	{0x23F0, 0x23F0, 2},   // alarm clock
	{0x23F3, 0x23F3, 2},   // hourglass
	{0x25FD, 0x25FE, 2},   // small squares
	{0x2614, 0x2615, 2},   // umbrella, hot beverage
	{0x2630, 0x2637, 2},   // trigrams
	{0x2648, 0x2653, 2},   // zodiac
	{0x267F, 0x267F, 2},   // wheelchair
	{0x268A, 0x268F, 2},   // monograms
	{0x2693, 0x2693, 2},   // anchor
	{0x26A1, 0x26A1, 2},   // high voltage
	{0x26AA, 0x26AB, 2},   // circles
	{0x26BD, 0x26BE, 2},   // soccer, baseball
	{0x26C4, 0x26C5, 2},   // snowman
	{0x26CE, 0x26CE, 2},   // ophiuchus
	{0x26D4, 0x26D4, 2},   // no entry
	{0x26EA, 0x26EA, 2},   // church
	{0x26F2, 0x26F3, 2},   // fountain, flag
	{0x26F5, 0x26F5, 2},   // sailboat
	{0x26FA, 0x26FA, 2},   // tent
	{0x26FD, 0x26FD, 2},   // fuel pump
	{0x2705, 0x2705, 2},   // white heavy check mark (status OK icon)
	{0x270A, 0x270B, 2},   // fist
	{0x2728, 0x2728, 2},   // sparkles
	{0x274C, 0x274C, 2},   // cross mark
	{0x274E, 0x274E, 2},   // negative squared cross mark
	{0x2753, 0x2755, 2},   // question marks
	{0x2757, 0x2757, 2},   // exclamation mark
	{0x2795, 0x2797, 2},   // plus/minus/divide
	{0x27B0, 0x27B0, 2},   // curl loop
	{0x27BF, 0x27BF, 2},   // double curl loop
	{0x2B1B, 0x2B1C, 2},   // large squares
	{0x2B50, 0x2B50, 2},   // star
	{0x2B55, 0x2B55, 2},   // heavy circle
	{0x2E80, 0x303E, 2},   // CJK radicals .. Kangxi
	{0x3041, 0x33FF, 2},   // Hiragana .. CJK compatibility
	{0x3400, 0x4DBF, 2},   // CJK ext A
	{0x4E00, 0x9FFF, 2},   // CJK unified ideographs
	{0xA000, 0xA4CF, 2},   // Yi
	{0xA960, 0xA97F, 2},   // Hangul Jamo Extended-A
	{0xAC00, 0xD7A3, 2},   // Hangul syllables
	{0xF900, 0xFAFF, 2},   // CJK compatibility ideographs
	{0xFE10, 0xFE19, 2},   // vertical forms
	{0xFE30, 0xFE6F, 2},   // CJK compatibility forms
	{0xFF00, 0xFF60, 2},   // fullwidth forms
	{0xFFE0, 0xFFE6, 2},   // fullwidth signs
	{0x1B000, 0x1B001, 2}, // Kana supplement
	// Emoji and pictographs: only codepoints with Unicode
	// Emoji_Presentation=Yes measure 2. The ranges mirror go-runewidth's
	// doublewidth table exactly, so text-presentation pictographs such as
	// U+1F5A5 (desktop) measure 1 like the rest of the ecosystem.
	{0x1F004, 0x1F004, 2}, // mahjong red dragon
	{0x1F0CF, 0x1F0CF, 2}, // joker
	{0x1F18E, 0x1F18E, 2}, // AB button
	{0x1F191, 0x1F19A, 2}, // squared CL..VS
	{0x1F200, 0x1F202, 2}, // squared katakana
	{0x1F210, 0x1F23B, 2}, // squared CJK
	{0x1F240, 0x1F248, 2}, // tortoise shell bracketed CJK
	{0x1F250, 0x1F251, 2}, // circled ideographs
	{0x1F260, 0x1F265, 2}, // rounded symbols
	{0x1F300, 0x1F320, 2}, // weather, landscape, animals
	{0x1F32D, 0x1F335, 2}, // food
	{0x1F337, 0x1F37C, 2}, // plants and food
	{0x1F37E, 0x1F393, 2}, // drinks and celebration
	{0x1F3A0, 0x1F3CA, 2}, // entertainment and sport
	{0x1F3CF, 0x1F3D3, 2}, // sport
	{0x1F3E0, 0x1F3F0, 2}, // buildings
	{0x1F3F4, 0x1F3F4, 2}, // black flag
	{0x1F3F8, 0x1F43E, 2}, // sport, transport, animals
	{0x1F440, 0x1F440, 2}, // eyes
	{0x1F442, 0x1F4FC, 2}, // body, people, objects
	{0x1F4FF, 0x1F53D, 2}, // objects and symbols
	{0x1F54B, 0x1F54E, 2}, // religious symbols
	{0x1F550, 0x1F567, 2}, // clock faces
	{0x1F57A, 0x1F57A, 2}, // dancing man
	{0x1F595, 0x1F596, 2}, // hand gestures
	{0x1F5A4, 0x1F5A4, 2}, // black heart
	{0x1F5FB, 0x1F64F, 2}, // pictographs and emoticons
	{0x1F680, 0x1F6C5, 2}, // transport and map symbols
	{0x1F6CC, 0x1F6CC, 2}, // person in bed
	{0x1F6D0, 0x1F6D2, 2}, // symbols
	{0x1F6D5, 0x1F6D8, 2}, // symbols
	{0x1F6DC, 0x1F6DF, 2}, // symbols
	{0x1F6EB, 0x1F6EC, 2}, // airplanes
	{0x1F6F4, 0x1F6FC, 2}, // transport
	{0x1F7E0, 0x1F7EB, 2}, // colored circles and squares
	{0x1F7F0, 0x1F7F0, 2}, // heavy equals sign
	{0x1F90C, 0x1F93A, 2}, // supplemental symbols
	{0x1F93C, 0x1F945, 2}, // supplemental symbols
	{0x1F947, 0x1F9FF, 2}, // supplemental symbols
	{0x1FA70, 0x1FA7C, 2}, // symbols and pictographs extended-A
	{0x1FA80, 0x1FA8A, 2}, // symbols and pictographs extended-A
	{0x1FA8E, 0x1FAC6, 2}, // symbols and pictographs extended-A
	{0x1FAC8, 0x1FAC8, 2}, // symbols and pictographs extended-A
	{0x1FACD, 0x1FADC, 2}, // symbols and pictographs extended-A
	{0x1FADF, 0x1FAEA, 2}, // symbols and pictographs extended-A
	{0x1FAEF, 0x1FAF8, 2}, // symbols and pictographs extended-A
	{0x20000, 0x2FFFD, 2}, // CJK ext B+
	{0x30000, 0x3FFFD, 2}, // CJK ext G+
}
