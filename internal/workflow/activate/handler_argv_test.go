package activate

import (
	"context"
	"os/exec"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/structdefault"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newActivateMachine builds a LOCAL machine with a controlled runtime
// IsRoot, so recorded argv shows privilege prefixes directly.
func newActivateMachine(t *testing.T, isRoot bool) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.Bootstrap.DisableAutomaticReboot = true
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: isRoot})

	structdefault.Apply(&mach.Attributes)

	return mach
}

// MaybeSudo prefix for a non-root SSH user, none for root; the env(1)
// argv rides inside the prefix so variables survive sudo's env_reset.
func TestExecuteBootstrap_NixosInstallElevation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		isRoot bool
		want   string
	}{
		{
			name:   "non-root SSH user: sudo prefix, env argv inside",
			isRoot: false,
			want:   "sudo env NIXOS_INSTALL_ENV=test nixos-install --no-root-passwd --no-channel-copy --system /nix/store/closure --root /mnt",
		},
		{
			name:   "root SSH user: bare command",
			isRoot: true,
			want:   "env NIXOS_INSTALL_ENV=test nixos-install --no-root-passwd --no-channel-copy --system /nix/store/closure --root /mnt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newActivateMachine(t, tt.isRoot)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			nixCfg := &nix.NixConfig{Env: map[string]string{"NIXOS_INSTALL_ENV": "test"}}
			require.NoError(t, nixCfg.Init(nil))

			require.NoError(t, executeBootstrap(exc, mach, nixCfg, "/nix/store/closure"))

			assert.Equal(t, tt.want, testutil.LastCommandLine(t, phaseLog))
		})
	}
}

// nixos-install accepts a command prefix override, and a configured executable
// path is used verbatim, without a PATH lookup.
func TestExecuteBootstrap_NixosInstallCommandOverride(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{NixosInstallCommand: []string{"/run/wrappers/bin/nixos-install", "--verbose"}}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, executeBootstrap(exc, mach, nixCfg, "/nix/store/closure"))

	assert.Equal(t, "/run/wrappers/bin/nixos-install --verbose "+
		"--no-root-passwd --no-channel-copy --system /nix/store/closure --root /mnt",
		testutil.LastCommandLine(t, phaseLog))
}

// The system closure and target root are configured flag values now, so a
// replacement installer can order them as its own CLI expects. The built-in
// default keeps the shape the readback expects.
func TestExecuteBootstrap_NixosInstallDefaultFlags(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{
		NixosInstallDefaultFlags: []string{"--root", "/mnt", "--system", "$PANIX_CLOSURE", "--no-channel-copy"},
	}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, executeBootstrap(exc, mach, nixCfg, "/nix/store/closure"))

	assert.Equal(t, "nixos-install --root /mnt --system /nix/store/closure --no-channel-copy",
		testutil.LastCommandLine(t, phaseLog))
}

// The built-in installer must receive the closure panix built. Dropping it sends
// nixos-install to whatever it finds in the target's /etc/nixos, and the mistake
// would only surface after the disk was written.
func TestExecuteBootstrap_DefaultFlagsWithoutClosureRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		flags []string
	}{
		{name: "empty list", flags: []string{}},
		{name: "flags without the closure", flags: []string{"--no-root-passwd", "--no-channel-copy"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mach := newActivateMachine(t, true)
			exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

			nixCfg := &nix.NixConfig{NixosInstallDefaultFlags: tt.flags}
			require.NoError(t, nixCfg.Init(nil))

			err := executeBootstrap(exc, mach, nixCfg, "/nix/store/closure")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "dropped the built closure")
			assert.Empty(t, testutil.CommandLines(t, phaseLog), "the install must not run at all")
		})
	}
}

// A replacement installer defines its own operands, so the build-method guard
// only applies to the built-in nixos-install.
func TestExecuteBootstrap_ReplacementInstallerNeedsNoBuildMethodFlag(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{
		NixosInstallCommand:      []string{"/run/wrappers/bin/nixos-install-ng"},
		NixosInstallDefaultFlags: []string{},
	}
	require.NoError(t, nixCfg.Init(nil))

	require.NoError(t, executeBootstrap(exc, mach, nixCfg, "/nix/store/closure"))

	assert.Equal(t, "/run/wrappers/bin/nixos-install-ng", testutil.LastCommandLine(t, phaseLog))
}

// A flag list referencing a value this step does not supply fails before
// anything runs, naming the field.
func TestExecuteBootstrap_UnavailableRuntimeVariableFails(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	exc, _ := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{NixosInstallDefaultFlags: []string{"--system", "$PANIX_PROFILE_PATH"}}
	require.NoError(t, nixCfg.Init(nil))

	err := executeBootstrap(exc, mach, nixCfg, "/nix/store/closure")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nixos_install_default_flags")
	assert.Contains(t, err.Error(), "PANIX_PROFILE_PATH")
}

// The readback comparison is the guard that keeps a no-op installer from
// reaching the reboot, so pin it directly: a dry run never runs the readback's
// OnSuccess.
func TestRequireInstalledClosure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		installed string
		wantErr   bool
	}{
		{name: "matching closure", installed: "/nix/store/closure"},
		{name: "different closure", installed: "/nix/store/other", wantErr: true},
		{name: "empty readback", installed: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := requireInstalledClosure(installedSystemProfile, tt.installed, "/nix/store/closure")

			if !tt.wantErr {
				require.NoError(t, err)

				return
			}

			require.ErrorContains(t, err, "refusing to reboot")
		})
	}
}

// A non-root SSH user needs elevation to read a root-owned profile link.
func TestVerifyInstalledSystem_Elevation(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, false)
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	require.NoError(t, verifyInstalledSystem(exc, mach, installedSystemProfile, "/nix/store/dry-run-closure"))

	assert.Equal(t, "sudo readlink -f /mnt/nix/var/nix/profiles/system", testutil.LastCommandLine(t, phaseLog))
}

// A NixConfig that skipped Init has no command. The built-in one is materialized
// here instead of indexing an empty slice, matching how the flag lists are read.
func TestNixosInstallArgv_UninitializedCommandFallsBackToBuiltin(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	exc, _ := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{
		NixosInstallDefaultFlags: []string{"--system", "$PANIX_CLOSURE"},
	}

	argv, _, err := nixosInstallArgv(exc, nixCfg, "/nix/store/closure")

	require.NoError(t, err)
	assert.Equal(t, []string{"nixos-install", "--system", "/nix/store/closure"}, argv)
}

// A wrapper prefix still runs the built-in installer, so the closure guard has to
// see through it: env(1) and sudo-style prefixes put nixos-install at any index.
func TestExecuteBootstrap_ClosureGuardSeesThroughWrapper(t *testing.T) {
	t.Parallel()

	mach := newActivateMachine(t, true)
	exc, _ := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	nixCfg := &nix.NixConfig{
		NixosInstallCommand:      []string{"env", "NIX_CONFIG=experimental-features = nix-command", "nixos-install"},
		NixosInstallDefaultFlags: []string{},
	}
	require.NoError(t, nixCfg.Init(nil))

	err := executeBootstrap(exc, mach, nixCfg, "/nix/store/closure")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dropped the built closure")
}

// reboot goes through ssh, and the host can go down before ssh delivers the
// remote exit status: ssh then reports its own connection-lost status. That is
// the expected outcome of a reboot, not a failure.
func TestIsSSHDisconnect(t *testing.T) {
	t.Parallel()

	exitWith := func(code string) error {
		//nolint:gosec // code is a test table constant
		return exec.CommandContext(context.Background(), "sh", "-c", "exit "+code).Run()
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "ssh connection-lost status", err: exitWith("255"), want: true},
		{name: "wrapped connection-lost status", err: errors.Wrap(exitWith("255"), "command failed"), want: true},
		{name: "remote command failure", err: exitWith("1")},
		{name: "command could not start", err: &exec.Error{Name: "ssh", Err: exec.ErrNotFound}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isSSHDisconnect(tt.err))
		})
	}
}
