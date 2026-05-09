package workflow

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildOnSuccessStorePathValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupBuf func(buf *linesbuffer.LinesBuffer)
		wantPath string
		wantErr  bool
	}{
		{
			"store path after CR overwrite",
			func(buf *linesbuffer.LinesBuffer) {
				buf.Write([]byte("[1/1/2 built] building disko"))
				buf.OverrideLastLine([]byte("/nix/store/abc-disko"))
			},
			"/nix/store/abc-disko", false,
		},
		{
			"ANSI stripped from last line",
			func(buf *linesbuffer.LinesBuffer) {
				buf.OverrideLastLine([]byte("\x1b[0m/nix/store/xyz\x1b[0m"))
			},
			"/nix/store/xyz", false,
		},
		{"empty output", func(buf *linesbuffer.LinesBuffer) {}, "", true},
		{
			"no store path prefix",
			func(buf *linesbuffer.LinesBuffer) {
				buf.Write([]byte("build failed with exit code 1"))
			},
			"", true,
		},
		{
			"warning then store path",
			func(buf *linesbuffer.LinesBuffer) {
				buf.Write([]byte("warning: dirty tree"))
				buf.Write([]byte("/nix/store/abc-disko"))
			},
			"/nix/store/abc-disko", false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cl := &command.CommandLog{Output: linesbuffer.New()}
			tt.setupBuf(cl.Output)

			storePath := style.StripANSI(string(cl.Output.LastLine()))
			isValid := storePath != "" && strings.HasPrefix(storePath, "/nix/store/")

			if tt.wantErr {
				require.False(t, isValid, "expected validation fail for: %q", storePath)
			} else {
				require.True(t, isValid, "expected valid store path, got: %q", storePath)
				assert.Equal(t, tt.wantPath, storePath)
			}
		})
	}
}
