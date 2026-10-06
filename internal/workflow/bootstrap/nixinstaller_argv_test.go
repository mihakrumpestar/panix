package bootstrap

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const chmodNixInstallerLine = "chmod +x /tmp/panix-nix-installer"

// The staged fetch uses the shared downloadOrTransfer helper: an http(s) URL
// is downloaded with curl on the target (with bootstrap.nix.curl_default_flags,
// falling back to the shared curl defaults), a local path is transferred with
// rsync. The fetch contract is "staged and runnable": every transfer is
// followed by `chmod +x` on the staged path. The curl argv is the pure
// downloadArgv helper's output.
func TestFetchNixInstaller_Argv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		nixCfg *attributes.NixConfig
		source string
		want   string
	}{
		{
			name:   "url fetched with the default curl flags",
			source: "https://install.determinate.systems/nix",
			want:   "curl --fail -# -L -C - https://install.determinate.systems/nix -o /tmp/panix-nix-installer",
		},
		{
			name:   "local shebang script source transferred",
			source: "./nix-installer.sh",
			want:   "rsync -rcPEx --mkpath --chmod=D700,F700 ./nix-installer.sh /tmp/panix-nix-installer",
		},
		{
			name:   "local installer binary source transferred",
			source: "./nix-installer",
			want:   "rsync -rcPEx --mkpath --chmod=D700,F700 ./nix-installer /tmp/panix-nix-installer",
		},
		{
			name:   "url fetched with the configured curl flags",
			nixCfg: &attributes.NixConfig{CurlDefaultFlags: []string{"-fsSL"}},
			source: "https://install.determinate.systems/nix",
			want:   "curl -fsSL https://install.determinate.systems/nix -o /tmp/panix-nix-installer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, true)
			mach.Bootstrap.Nix = tt.nixCfg
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			require.NoError(t, fetchNixInstaller(exc, mach, tt.source))

			// Staged and runnable: transfer, then chmod before any run.
			assert.Equal(t, []string{tt.want, chmodNixInstallerLine}, testutil.CommandLines(t, phaseLog))
		})
	}
}

// The installer runs elevated (it writes the nix store and profiles
// system-wide) and its args fully replace the defaults.
func TestRunNixInstaller_Argv(t *testing.T) {
	t.Parallel()

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
			want:   "sudo /tmp/panix-nix-installer install --no-confirm",
		},
		{
			name:   "root, unset bootstrap.nix: tagged defaults",
			isRoot: true,
			nixCfg: nil,
			want:   "/tmp/panix-nix-installer install --no-confirm",
		},
		{
			name:   "custom args fully replace defaults",
			isRoot: false,
			nixCfg: &attributes.NixConfig{Args: []string{"install", "--extra-flag"}},
			want:   "sudo /tmp/panix-nix-installer install --extra-flag",
		},
		{
			name:   "explicit empty args run the bare installer",
			isRoot: true,
			nixCfg: &attributes.NixConfig{Args: []string{}},
			want:   "/tmp/panix-nix-installer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newKexecMachine(t, tt.isRoot)
			mach.Bootstrap.Nix = tt.nixCfg
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Bootstrap)

			require.NoError(t, runNixInstaller(exc, mach))

			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}

// The runner never inspects the staged file: a shebang shell script source
// and an installer binary source (e.g. the nix-installer release binary)
// produce the identical run argv. The kernel runs the script via its shebang
// and the binary natively.
func TestInstallNix_SourceKindDoesNotChangeRunArgv(t *testing.T) {
	t.Parallel()

	const wantRun = "/tmp/panix-nix-installer install --no-confirm"

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
			require.Len(t, lines, 4)
			assert.Equal(t, []string{chmodNixInstallerLine, wantRun}, lines[1:3],
				"fetch must chmod before the run, the run argv must not depend on the source kind")
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
