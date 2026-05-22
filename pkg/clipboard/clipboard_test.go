package clipboard

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripANSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"NoANSI", "plain text", "plain text"},
		{"SingleANSI", "\x1b[31mred\x1b[0m", "red"},
		{"MultipleANSI", "\x1b[1;32mgreen\x1b[0m \x1b[34mblue\x1b[0m", "green blue"},
		{"ComplexANSI", "\x1b[38;2;255;0;0mRGB\x1b[0m", "RGB"},
		{"Empty", "", ""},
		{"OnlyANSI", "\x1b[31m\x1b[0m", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, string(style.StripANSI([]byte(test.input))))
		})
	}
}

func TestNormalizeText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"Plain", "hello", "hello"},
		{"WithWhitespace", "  hello world  ", "hello world"},
		{"WithANSI", "\x1b[31mred\x1b[0m", "red"},
		{"Mixed", "  \x1b[1mbold\x1b[0m  ", "bold"},
		{"Newlines", "\n\nhello\n", "hello"},
		{"Tabs", "\t\thello\t", "hello"},
		{"Empty", "", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, normalizeText(test.input))
		})
	}
}

func TestWriteOSC52(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"Simple", "hello"},
		{"WithSpaces", "hello world"},
		{"Multiline", "line1\nline2\nline3"},
		{"Special", "special!@#$%^&*()"},
		{"Unicode", "\u65e5\u672c\u8a9e"},
		{"Empty", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			normalized := normalizeText(test.input)

			var buf bytes.Buffer

			err := writeOSC52(&buf, normalized)

			require.NoError(t, err, "writeOSC52(%q)", normalized)

			output := buf.String()
			prefix := "\x1b]52;"
			suffix := "\x07"

			assert.True(t, strings.HasPrefix(output, prefix), "OSC52 output missing prefix %q: %q", prefix, output)
			assert.True(t, strings.HasSuffix(output, suffix), "OSC52 output missing suffix %q: %q", suffix, output)

			encoded := strings.TrimPrefix(strings.TrimSuffix(output, suffix), prefix)

			decoded, err := base64.StdEncoding.DecodeString(encoded)
			require.NoError(t, err, "failed to decode OSC52 base64")
			assert.Equal(t, normalized, string(decoded))
		})
	}
}
