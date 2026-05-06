package clipboard

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
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

			got := stripANSI(test.input)
			if got != test.want {
				t.Errorf("stripANSI(%q) = %q, want %q", test.input, got, test.want)
			}
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

			got := normalizeText(test.input)
			if got != test.want {
				t.Errorf("normalizeText(%q) = %q, want %q", test.input, got, test.want)
			}
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
			if err != nil {
				t.Errorf("writeOSC52(%q) error: %v", normalized, err)

				return
			}

			output := buf.String()

			prefix := "\x1b]52;"
			suffix := "\x07"

			if !strings.HasPrefix(output, prefix) {
				t.Errorf("OSC52 output missing prefix %q, got %q", prefix, output)

				return
			}

			if !strings.HasSuffix(output, suffix) {
				t.Errorf("OSC52 output missing suffix %q, got %q", suffix, output)

				return
			}

			encoded := strings.TrimPrefix(strings.TrimSuffix(output, suffix), prefix)

			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Errorf("failed to decode OSC52 base64: %v", err)

				return
			}

			if string(decoded) != normalized {
				t.Errorf("OSC52 decoded = %q, want %q", string(decoded), normalized)
			}
		})
	}
}
