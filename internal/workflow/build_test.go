package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLastNonEmptyLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{"empty input", []byte{}, []byte{}},
		{"nil input", nil, []byte{}},
		{"single line", []byte("single line"), []byte("single line")},
		{"trailing newlines", []byte("output\n\n\n"), []byte("output")},
		{"multiple trailing newlines", []byte("first\nsecond\n\n\n"), []byte("second")},
		{"only whitespace", []byte("  \n  \n  "), []byte{}},
		{"only newlines", []byte("\n\n\n"), []byte{}},
		{"content with leading whitespace", []byte("\n\n  content  \n"), []byte("content")},
		{
			"realistic nix build output",
			[]byte("warning: Git tree is dirty\nevaluating flake...\n/nix/store/xyz\n"),
			[]byte("/nix/store/xyz"),
		},
		{"line with tabs", []byte("line1\n\t  line2 with spaces  \t\n"), []byte("line2 with spaces")},
		{"windows line endings", []byte("line1\r\nline2\r\n"), []byte("line2")},
		{"single trailing newline", []byte("single\n"), []byte("single")},
		{
			"carriage return only treated as single line",
			[]byte("line1\rline2"),
			[]byte("line1\rline2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, lastNonEmptyLine(tt.input))
		})
	}
}
