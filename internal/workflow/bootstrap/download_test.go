package bootstrap

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/urlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// downloadOrTransfer dispatch: an unsupported scheme fails before any command
// runs, an http(s) URL downloads with curl on the target using the passed
// flags, a local path transfers with rsync.
//
//nolint:funlen
func TestDownloadOrTransfer_SourceDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		source    string
		curlFlags []string
		wantErrIs error
		wantLine  string
	}{
		{
			name:      "https url downloaded with curl and the passed flags",
			source:    "https://example.com/installer.sh",
			curlFlags: []string{"-fsSL"},
			wantLine:  "curl -fsSL https://example.com/installer.sh -o /tmp/remote/installer.sh",
		},
		{
			name:      "http url downloaded with curl",
			source:    "http://example.com/installer.sh",
			curlFlags: []string{"--fail", "-#", "-L", "-C", "-"},
			wantLine:  "curl --fail -# -L -C - http://example.com/installer.sh -o /tmp/remote/installer.sh",
		},
		{
			name:     "local path transferred with rsync",
			source:   "./installer.sh",
			wantLine: "rsync -rcPEx --mkpath --chmod=D700,F700 ./installer.sh /tmp/remote/installer.sh",
		},
		{
			name:      "ftp scheme rejected before curl or rsync",
			source:    "ftp://host/installer.sh",
			curlFlags: []string{"-fsSL"},
			wantErrIs: urlx.ErrUnsupportedURLScheme,
		},
		{
			name:      "file scheme rejected before curl or rsync",
			source:    "file:///installer.sh",
			curlFlags: []string{"-fsSL"},
			wantErrIs: urlx.ErrUnsupportedURLScheme,
		},
		{
			name:      "scheme typo rejected before curl or rsync",
			source:    "htps://host/installer.sh",
			curlFlags: []string{"-fsSL"},
			wantErrIs: urlx.ErrUnsupportedURLScheme,
		},
		{
			name:      "s3 scheme rejected before curl or rsync",
			source:    "s3://bucket/key",
			curlFlags: []string{"-fsSL"},
			wantErrIs: urlx.ErrUnsupportedURLScheme,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, true)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			err := downloadOrTransfer(exc, mach, tt.source, "/tmp/remote/installer.sh", "installer", tt.curlFlags)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Empty(t, testutil.CommandLines(t, phaseLog), "no command may run for an invalid source")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantLine, testutil.LastCommandLine(t, phaseLog))
		})
	}
}
