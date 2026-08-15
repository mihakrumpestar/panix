package style

// runeWidth returns the terminal cell width of a single rune.
//
// This is a replacement for rivo/uniseg's grapheme-cluster width.
// The TUI renders terminal content (Nix build logs, tree connectors, braille
// spinners) plus a fixed set of hardcoded emoji icons, so grapheme-cluster
// segmentation (ZWJ sequences, skin-tone modifiers) is unnecessary.
//
// Rules:
//   - control characters: width 0
//   - combining marks, variation selectors, zero-width controls: width 0
//   - East Asian wide/fullwidth runes (CJK, Hangul, fullwidth forms): width 2
//   - emoji (U+1F300+ blocks plus the emoji-presentation symbols used by the
//     color scheme): width 2
//   - everything else (ASCII printable, box drawing, braille, ambiguous-width
//     symbols like gear/ballot-x): width 1
func runeWidth(r rune) int {
	for _, entry := range widthRanges {
		if r >= entry.lo && r <= entry.hi {
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
	{0x2648, 0x2653, 2},   // zodiac
	{0x267F, 0x267F, 2},   // wheelchair
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
	{0x1F300, 0x1F64F, 2}, // emoji
	{0x1F680, 0x1F6FF, 2}, // transport and map symbols
	{0x1F900, 0x1F9FF, 2}, // supplemental symbols
	{0x1FA70, 0x1FAFF, 2}, // symbols and pictographs extended
	{0x20000, 0x2FFFD, 2}, // CJK ext B+
	{0x30000, 0x3FFFD, 2}, // CJK ext G+
}
