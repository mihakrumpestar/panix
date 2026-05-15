package workflow

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildOnSuccessStorePathValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupBuf func(buf *buffer.LinesBufVer)
		wantPath string
		wantErr  bool
	}{
		{
			"store path after CR overwrite",
			func(buf *buffer.LinesBufVer) {
				buf.Write([]byte("[1/1/2 built] building disko"))
				buf.OverrideLastLine([]byte("/nix/store/abc-disko"))
			},
			"/nix/store/abc-disko", false,
		},
		{
			"ANSI stripped from last line",
			func(buf *buffer.LinesBufVer) {
				buf.OverrideLastLine([]byte("\x1b[0m/nix/store/xyz\x1b[0m"))
			},
			"/nix/store/xyz", false,
		},
		{"empty output", func(buf *buffer.LinesBufVer) {}, "", true},
		{
			"no store path prefix",
			func(buf *buffer.LinesBufVer) {
				buf.Write([]byte("build failed with exit code 1"))
			},
			"", true,
		},
		{
			"warning then store path",
			func(buf *buffer.LinesBufVer) {
				buf.Write([]byte("warning: dirty tree"))
				buf.Write([]byte("/nix/store/abc-disko"))
			},
			"/nix/store/abc-disko", false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cl := &command.CommandLog{Output: buffer.NewLinesBufVer()}
			test.setupBuf(cl.Output)

			storePath := string(style.StripANSI(cl.Output.LastLine()))
			isValid := len(storePath) > 0 && strings.HasPrefix(storePath, "/nix/store/")

			if test.wantErr {
				require.False(t, isValid, "expected validation fail for: %q", storePath)
			} else {
				require.True(t, isValid, "expected valid store path, got: %q", storePath)
				assert.Equal(t, test.wantPath, storePath)
			}
		})
	}
}
