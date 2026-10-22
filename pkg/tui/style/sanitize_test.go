package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeTerminalOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text", "hello world", "hello world"},
		{"newline tab carriage return and backspace kept", "a\nb\tc\rd\be", "a\nb\tc\rd\be"},
		{"SGR kept", "\x1b[31mred\x1b[0m", "\x1b[31mred\x1b[0m"},
		{"truecolor SGR kept", "\x1b[38;2;1;2;3mx\x1b[m", "\x1b[38;2;1;2;3mx\x1b[m"},
		{"OSC 52 clipboard stripped (BEL)", "a\x1b]52;c;c2VjcmV0\x07b", "ab"},
		{"OSC 52 clipboard stripped (ST)", "a\x1b]52;c;c2VjcmV0\x1b\\b", "ab"},
		{"OSC 8 hyperlink stripped", "a\x1b]8;;https://example.com\x1b\\click\x1b]8;;\x1b\\b", "aclickb"},
		{"DCS stripped", "a\x1bP1;2|payload\x1b\\b", "ab"},
		{"SOS stripped", "a\x1bXpayload\x1b\\b", "ab"},
		{"PM stripped", "a\x1b^payload\x1b\\b", "ab"},
		{"APC stripped", "a\x1b_payload\x1b\\b", "ab"},
		{"CSI cursor and erase stripped", "a\x1b[2J\x1b[1;1H\x1b[Kb", "ab"},
		{"DEC private mode stripped", "a\x1b[?25lb\x1b[?25h", "ab"},
		{"bare two-byte escape stripped", "a\x1bMb", "ab"},
		{"stray ESC before printable keeps the printable", "a\x1bh", "ah"},
		{"C0 controls stripped", "a\x00b\x07c\x7fd", "abcd"},
		{"invalid UTF-8 stripped", string([]byte{'a', 0xFF, 'b'}), "ab"},
		{"Unicode C1 control stripped", "a\u0085b", "ab"},
		{"printable Unicode kept", "héllo 📦", "héllo 📦"},
		{"incomplete CSI dropped", "a\x1b[31", "a"},
		{"lone trailing ESC dropped", "a\x1b", "a"},
		{"unterminated OSC dropped", "a\x1b]52;c;secret", "a"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := SanitizeTerminalOutput(nil, []byte(test.in))
			assert.Equal(t, test.want, string(got), "input %q", test.in)
		})
	}
}

func TestSanitizeTerminalOutput_ReusesDestination(t *testing.T) {
	t.Parallel()

	dst := []byte("prefix:")
	got := SanitizeTerminalOutput(dst, []byte("\x1b[31mred\x1b]52;c;secret\x07"))

	assert.Equal(t, "prefix:\x1b[31mred", string(got))
}

func FuzzSanitizeTerminalOutput(f *testing.F) {
	for _, seed := range []string{
		"plain text\n\t",
		"\x1b[31mred\x1b[0m",
		"\x1b]52;c;c2VjcmV0\x07",
		"\x1b]8;;https://example.com\x1b\\click\x1b]8;;\x1b\\",
		"\x1bPpayload\x1b\\",
		"\x1b[2J\x1b[1;1H",
		"\x1b[38;2;1;2;3mx",
		"a\x1bh",
		"\x1b",
		"héllo 📦",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, in []byte) {
		out := SanitizeTerminalOutput(nil, in)

		assertOnlySGR(t, out)
		assert.Equal(t, out, SanitizeTerminalOutput(nil, out), "sanitizer must be idempotent")
	})
}

// assertOnlySGR checks that every ESC in out introduces a complete SGR
// sequence and that no control byte outside the allowed set survives.
func assertOnlySGR(t *testing.T, out []byte) {
	t.Helper()

	for pos := 0; pos < len(out); pos++ {
		byteI := out[pos]

		if byteI == '\x1b' {
			pos = requireSGRSequence(t, out, pos)

			continue
		}

		if byteI < 0x20 {
			require.True(t, isAllowedOutputControl(byteI), "control byte 0x%02x survived in %q", byteI, out)
		}

		require.NotEqual(t, byte(0x7F), byteI, "DEL survived in %q", out)
	}
}

// requireSGRSequence asserts that the escape sequence starting at pos is a
// complete SGR sequence and returns the position of its final byte.
func requireSGRSequence(t *testing.T, out []byte, pos int) int {
	t.Helper()

	require.Less(t, pos+1, len(out), "truncated ESC in %q", out)
	require.Equal(t, byte('['), out[pos+1], "only CSI may survive in %q", out)

	end := pos + 2
	for end < len(out) && out[end] >= 0x20 && out[end] <= 0x3F {
		end++
	}

	require.Less(t, end, len(out), "truncated CSI in %q", out)
	require.Equal(t, byte('m'), out[end], "only SGR may survive in %q", out)

	return end
}

// isAllowedOutputControl mirrors the sanitizer's allowed control bytes.
func isAllowedOutputControl(b byte) bool {
	switch b {
	case '\n', '\t', '\r', '\b':
		return true
	default:
		return false
	}
}
