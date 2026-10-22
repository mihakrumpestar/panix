package bootstrap

import (
	"path"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/urlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fetchNixInstallerCase struct {
	name      string
	nixCfg    *attributes.NixConfig
	source    string
	wantXfer  string
	wantErrIs error
}

// The staged fetch creates a unique private directory, transfers into it,
// then marks the staged file executable: an https URL is downloaded with curl
// (resume flags stripped), a local path is transferred with rsync.
func TestFetchNixInstaller_Argv(t *testing.T) {
	t.Parallel()

	tests := []fetchNixInstallerCase{
		{
			name:     "url fetched with the default curl flags",
			source:   "https://install.determinate.systems/nix",
			wantXfer: "curl --fail -# -L --proto =https --proto-redir =https https://install.determinate.systems/nix -o ",
		},
		{
			name:     "local shebang script source transferred",
			source:   "./nix-installer.sh",
			wantXfer: "rsync -rcEx --mkpath --perms --chmod=D700,F700 ./nix-installer.sh ",
		},
		{
			name:     "local installer binary source transferred",
			source:   "./nix-installer",
			wantXfer: "rsync -rcEx --mkpath --perms --chmod=D700,F700 ./nix-installer ",
		},
		{
			name:     "url fetched with the configured curl flags",
			nixCfg:   &attributes.NixConfig{CurlDefaultFlags: []string{"-fsSL"}},
			source:   "https://install.determinate.systems/nix",
			wantXfer: "curl -fsSL --proto =https --proto-redir =https https://install.determinate.systems/nix -o ",
		},
		{
			name:     "configured resume flags are stripped for a fresh file",
			nixCfg:   &attributes.NixConfig{CurlDefaultFlags: []string{"--fail", "-#", "-L", "-C", "-"}},
			source:   "https://install.determinate.systems/nix",
			wantXfer: "curl --fail -# -L --proto =https --proto-redir =https https://install.determinate.systems/nix -o ",
		},
		{
			name:      "plain http source rejected",
			source:    "http://install.determinate.systems/nix",
			wantErrIs: urlx.ErrInsecureURLScheme,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertFetchNixInstallerArgv(t, tt)
		})
	}
}

func assertFetchNixInstallerArgv(t *testing.T, tt fetchNixInstallerCase) {
	t.Helper()

	mach := newKexecMachine(t, true)
	mach.Bootstrap.Nix = tt.nixCfg
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	installerPath, err := fetchNixInstaller(exc, mach, tt.source)

	if tt.wantErrIs != nil {
		require.ErrorIs(t, err, tt.wantErrIs)
		assert.Empty(t, installerPath)
		assert.NotContains(t, strings.Join(testutil.CommandLines(t, phaseLog), "\n"), "curl",
			"an insecure source must not reach curl")

		return
	}

	require.NoError(t, err)

	stagingDir := path.Dir(installerPath)
	assert.True(t, strings.HasPrefix(stagingDir, "/tmp/panix-installer-"), installerPath)
	assert.Equal(t, installerFileName, path.Base(installerPath))

	lines := testutil.CommandLines(t, phaseLog)
	require.Len(t, lines, 3)
	assert.Equal(t, "install -d -m 700 -o kexec-user "+stagingDir, lines[0])
	assert.Equal(t, tt.wantXfer+installerPath, lines[1])
	assert.Equal(t, "chmod +x "+installerPath, lines[2])

	assert.NotContains(t, strings.Join(lines, "\n"), "/tmp/panix-nix-installer")
	assert.NotContains(t, strings.Join(lines, "\n"), "/tmp/kexec")
}

// The installer runs elevated (it writes the nix store and profiles
// system-wide) and its args fully replace the defaults.
func TestRunNixInstaller_Argv(t *testing.T) {
	t.Parallel()

	const installerPath = "/tmp/panix-installer-test/installer"

	tests := []struct {
		name   string
		isRoot bool
		nixCfg *attributes.NixConfig
		want   string
	}{
		{
			name:   "non-root default args: sudo prefix",
			isRoot: false,
			nixCfg: &attributes.NixConfig{},
			want:   "sudo " + installerPath + " install --no-confirm",
		},
		{
			name:   "root, unset bootstrap.nix: tagged defaults",
			isRoot: true,
			nixCfg: nil,
			want:   installerPath + " install --no-confirm",
		},
		{
			name:   "custom args fully replace defaults",
			isRoot: false,
			nixCfg: &attributes.NixConfig{Args: []string{"install", "--extra-flag"}},
			want:   "sudo " + installerPath + " install --extra-flag",
		},
		{
			name:   "explicit empty args run the bare installer",
			isRoot: true,
			nixCfg: &attributes.NixConfig{Args: []string{}},
			want:   installerPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, tt.isRoot)
			mach.Bootstrap.Nix = tt.nixCfg
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			require.NoError(t, runNixInstaller(exc, mach, installerPath))

			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}

// The runner never inspects the staged file: a shebang shell script source
// and an installer binary source (e.g. the nix-installer release binary)
// produce the identical run argv. The kernel runs the script via its shebang
// and the binary natively. The whole flow uses one unique staging dir and
// cleans it up.
func TestInstallNix_SourceKindDoesNotChangeRunArgv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
	}{
		{name: "shebang shell script source", source: "./nix-installer.sh"},
		{name: "elf installer binary source", source: "./nix-installer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, true)
			mach.Bootstrap.Nix = &attributes.NixConfig{URL: tt.source}
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			require.NoError(t, installNix(exc, mach))

			lines := testutil.CommandLines(t, phaseLog)
			require.Len(t, lines, 6)

			stagingDir := path.Dir(strings.TrimPrefix(lines[2], "chmod +x "))
			installerPath := path.Join(stagingDir, installerFileName)

			assert.Equal(t, "install -d -m 700 -o kexec-user "+stagingDir, lines[0])
			assert.Equal(t, "chmod +x "+installerPath, lines[2])
			assert.Equal(t, installerPath+" install --no-confirm", lines[3],
				"the run argv must not depend on the source kind")
			assert.Equal(t, "nix --version", lines[4])
			assert.Equal(t, "rm -rf -- "+stagingDir, lines[5], "the per-run staging dir must be cleaned up")

			assert.NotContains(t, strings.Join(lines, "\n"), "/tmp/panix-nix-installer")
			assert.NotContains(t, strings.Join(lines, "\n"), "/tmp/kexec")
		})
	}
}

// Verification is the shared nix probe: it runs `nix --version` and refreshes
// MetaInspect.NixAvailable for the later phases and skip decisions.
func TestVerifyNixInstallation_RefreshesNixAvailable(t *testing.T) {
	t.Parallel()

	mach := newKexecMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

	require.NoError(t, verifyNixInstallation(exc, mach))

	assert.Equal(t, "nix --version", testutil.LastCommandLine(t, phaseLog))
	assert.True(t, mach.MetaInspect.Load().NixAvailable)
}
