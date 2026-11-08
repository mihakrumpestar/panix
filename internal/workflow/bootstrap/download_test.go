package bootstrap

import (
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/structdefault"
	"github.com/mihakrumpestar/panix/pkg/urlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// downloadOrTransfer dispatch: an unsupported or insecure scheme fails before
// any command runs, an https URL downloads with curl on the target using the
// passed flags (minus resume flags), a local path transfers with rsync.
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
			wantLine:  "curl -fsSL --proto =https --proto-redir =https https://example.com/installer.sh -o /tmp/remote/installer.sh",
		},
		{
			name:      "https url strips resume flags",
			source:    "https://example.com/installer.sh",
			curlFlags: []string{"--fail", "-#", "-L", "-C", "-", "--continue-at", "100", "-C-", "--continue-at=42"},
			wantLine:  "curl --fail -# -L --proto =https --proto-redir =https https://example.com/installer.sh -o /tmp/remote/installer.sh",
		},
		{
			name:     "local path transferred with rsync",
			source:   "./installer.sh",
			wantLine: "rsync -rcEx --mkpath --perms --chmod=D700,F700 ./installer.sh /tmp/remote/installer.sh",
		},
		{
			name:      "plain http rejected as insecure before curl or rsync",
			source:    "http://example.com/installer.sh",
			curlFlags: []string{"--fail", "-#", "-L"},
			wantErrIs: urlx.ErrInsecureURLScheme,
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

			err := downloadOrTransfer(exc, mach, tt.source, "/tmp/remote/installer.sh", "installer", mach.Bootstrap.Kexec.CurlCommand, tt.curlFlags)

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

func TestDownloadArgv_ProtocolFlagsAfterUserFlags(t *testing.T) {
	t.Parallel()

	argv := downloadArgv(
		nil,
		[]string{"--fail", "--proto", "=http", "--proto-redir", "=http"},
		"https://example.com/installer.sh",
		"/tmp/remote/installer.sh",
	)

	wantTail := []string{
		"--proto", "=https",
		"--proto-redir", "=https",
		"https://example.com/installer.sh", "-o", "/tmp/remote/installer.sh",
	}
	require.GreaterOrEqual(t, len(argv), len(wantTail))
	assert.Equal(t, wantTail, argv[len(argv)-len(wantTail):],
		"the enforcing protocol flags must come after every user flag")

	userFlags := argv[:len(argv)-len(wantTail)]
	assert.Contains(t, userFlags, "--fail")
	assert.Contains(t, userFlags, "=http", "the user's permissive flag stays, then the tail overrides it")
}

func TestWithoutResumeFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		flags []string
		want  []string
	}{
		{"no flags", nil, []string{}},
		{"regular flags unchanged", []string{"--fail", "-#", "-L"}, []string{"--fail", "-#", "-L"}},
		{"short flag with value", []string{"-C", "-", "-L"}, []string{"-L"}},
		{"short flag attached", []string{"-C-", "-L"}, []string{"-L"}},
		{"long flag with value", []string{"--continue-at", "100", "-L"}, []string{"-L"}},
		{"long flag attached", []string{"--continue-at=100", "-L"}, []string{"-L"}},
		{"all resume forms", []string{"-C", "0", "--continue-at", "-", "-C-", "--continue-at=1"}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, withoutResumeFlags(tt.flags))
		})
	}
}

// A configured curl prefix replaces the downloader on a real call path: the
// Nix installer download must use bootstrap.nix.curl_command and the kexec
// download bootstrap.kexec.curl_command, and each must leave the other alone.
// The enforcing protocol flags stay appended last so an override cannot relax
// them.
func TestBootstrap_PerConsumerCurlCommand(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, true)
	mach.Bootstrap.Nix = attributes.NixConfig{
		URL:         "https://example.com/installer.sh",
		CurlCommand: []string{"nix-installer-curl"},
	}
	structdefault.Apply(&mach.Bootstrap.Nix)
	mach.Bootstrap.Kexec.CurlCommand = []string{"kexec-curl"}

	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	// The Nix installer path.
	_, err := fetchNixInstaller(exc, mach, "https://example.com/installer.sh")
	require.NoError(t, err)

	installerDownloads := downloadLines(testutil.CommandLines(t, phaseLog))
	require.Len(t, installerDownloads, 1)
	assert.Contains(t, installerDownloads[0], "nix-installer-curl")
	assert.NotContains(t, installerDownloads[0], "kexec-curl")

	// The kexec tarball path.
	lineCount := len(testutil.CommandLines(t, phaseLog))
	require.NoError(t, downloadOrTransferKexec(exc, mach, "https://example.com/kexec.tar.gz", "/tmp/panix-kexec-x"))

	kexecDownloads := downloadLines(testutil.CommandLines(t, phaseLog)[lineCount:])
	require.Len(t, kexecDownloads, 1)
	assert.Contains(t, kexecDownloads[0], "kexec-curl")
	assert.NotContains(t, kexecDownloads[0], "nix-installer-curl")
}

// downloadLines filters recorded commands down to the curl invocations, so an
// assertion cannot accidentally inspect a neighbouring step's command.
func downloadLines(lines []string) []string {
	var downloads []string

	for _, line := range lines {
		if strings.Contains(line, "--proto =https") {
			downloads = append(downloads, line)
		}
	}

	return downloads
}
