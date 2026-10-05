package clipboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
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

func TestClipboardPlan(t *testing.T) {
	t.Parallel()

	xclipArgs := []string{"-selection", "clipboard", "-in"}
	xselArgs := []string{"--clipboard", "--input"}

	tests := []struct {
		name      string
		goos      string
		isWayland bool
		want      []clipboardCommand
	}{
		{
			name: "darwin: pbcopy always ships on macOS",
			goos: "darwin",
			want: []clipboardCommand{{name: "pbcopy"}},
		},
		{
			name:      "linux wayland: wl-copy first, then the X11 tools",
			goos:      "linux",
			isWayland: true,
			want: []clipboardCommand{
				{name: "wl-copy", args: []string{"--"}},
				{name: "xclip", args: xclipArgs},
				{name: "xsel", args: xselArgs},
			},
		},
		{
			name: "linux x11: X11 tools only",
			goos: "linux",
			want: []clipboardCommand{
				{name: "xclip", args: xclipArgs},
				{name: "xsel", args: xselArgs},
			},
		},
		{
			name: "other platforms: no command, OSC52 handles them",
			goos: "windows",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, clipboardPlan(tt.goos, tt.isWayland))
		})
	}
}

// TestProbeClipboardCommands_LinuxFallsThrough pins the probe loop: a failing
// candidate falls through to the next one and the text reaches the successful
// command via stdin. The plan is built explicitly for linux (the CI machine is
// linux); the darwin plan is never executed here.
//
//nolint:paralleltest // t.Setenv forbids parallel tests
func TestProbeClipboardCommands_LinuxFallsThrough(t *testing.T) {
	dir := t.TempDir()
	stdinPath := filepath.Join(dir, "stdin.txt")

	failScript := "#!/bin/sh\nexit 1\n"
	//nolint:gosec // the stub must be executable to be found on PATH
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(failScript), 0o700))

	captureScript := "#!/bin/sh\ncat > '" + stdinPath + "'\n"
	//nolint:gosec // the stub must be executable to be found on PATH
	require.NoError(t, os.WriteFile(filepath.Join(dir, "xclip"), []byte(captureScript), 0o700))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	payload := "copy me"

	ok := probeClipboardCommands(context.Background(), clipboardPlan("linux", true), payload)

	require.True(t, ok, "the probe loop must fall through to xclip after wl-copy fails")

	written, err := os.ReadFile(stdinPath) //nolint:gosec // test-controlled temp path
	require.NoError(t, err)
	assert.Equal(t, payload, string(written), "stdin must reach the successful command")
}
