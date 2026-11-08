package phaseops

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRemoteMachine returns a machine initialized enough for SSH store URL
// generation (hostname, port, username, initialized state pointer).
func newRemoteMachine(t *testing.T, hostname string) *machine.Machine {
	t.Helper()

	machineI := &machine.Machine{}
	machineI.SSH.Hostname = hostname
	machineI.SSH.Port = 22
	machineI.SSH.Username = "root"
	machineI.State = atomicpointer.New[machine.State]()

	return machineI
}

// newRemoteInstallable returns a remote-mode installable owning the given
// machines in declaration order. Nix defaults are materialized as config init
// does, so the argv asserts see the built-in prefixes and flags.
func newRemoteInstallable(t *testing.T, machines ...*machine.Machine) *installable.Installable {
	t.Helper()

	inst := &installable.Installable{}
	inst.Nix.BuildMode = nix.BuildModeRemote
	require.NoError(t, inst.Nix.Init(nil))

	inst.Machines = atomicorderedmap.New[string, *machine.Machine]()

	for i, m := range machines {
		inst.Machines.Set(fmt.Sprintf("machine-%d", i), m)
	}

	return inst
}

// TestNixBuildCommand_RemoteModePinnedToFirstMachine is the regression test
// for remote builds racing: the Build phase is once-per-installable and any
// machine's goroutine may execute it, but the --store URL must always target
// the first declared machine, because the transfer phase copies the closure
// --from that same machine. The command takes no executing-machine input, so
// it is identical no matter which goroutine builds.
func TestNixBuildCommand_RemoteModePinnedToFirstMachine(t *testing.T) {
	t.Parallel()

	first := newRemoteMachine(t, "10.0.0.1")
	second := newRemoteMachine(t, "10.0.0.2")
	inst := newRemoteInstallable(t, first, second)

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, "")

	storeIdx := slices.Index(cmd, "--store")
	require.NotEqual(t, -1, storeIdx, "remote build command must contain --store: %v", cmd)
	assert.Equal(t, "ssh-ng://root@10.0.0.1:22", cmd[storeIdx+1])
}

// TestRemoteBuilderSSH_PanicsOnNoMachines verifies the invariant contract:
// validation rejects remote mode without machines, so reaching this state at
// runtime is an internal error that fails loudly (recovered into a phase
// error by onceasync/pool), mirroring Machine.GetActiveSSH.
func TestRemoteBuilderSSH_PanicsOnNoMachines(t *testing.T) {
	t.Parallel()

	inst := newRemoteInstallable(t)

	assert.PanicsWithValue(
		t,
		"internal error: remote-mode installable has no machines (validation should have rejected it)",
		func() { remoteBuilderSSH(inst) },
	)
}

func TestNixBuildCommand_LocalModeHasNoStoreFlag(t *testing.T) {
	t.Parallel()

	m := newRemoteMachine(t, "10.0.0.1")
	inst := newRemoteInstallable(t, m)
	inst.Nix.BuildMode = nix.BuildModeLocal

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, "")

	assert.Equal(t, -1, slices.Index(cmd, "--store"))
}

// TestNixCopyBaseArgs_RemoteModeCopiesFromFirstMachine verifies the transfer
// --from source stays pinned to the first declared machine, matching the
// build --store target (see TestNixBuildCommand_RemoteModePinnedToFirstMachine).
func TestNixCopyBaseArgs_RemoteModeCopiesFromFirstMachine(t *testing.T) {
	t.Parallel()

	first := newRemoteMachine(t, "10.0.0.1")
	second := newRemoteMachine(t, "10.0.0.2")
	inst := newRemoteInstallable(t, first, second)

	args, err := nixCopyBaseArgs(inst, "ssh-ng://root@10.0.0.2:22")
	require.NoError(t, err)

	fromIdx := slices.Index(args, "--from")
	require.NotEqual(t, -1, fromIdx, "remote copy command must contain --from: %v", args)
	assert.Equal(t, "ssh-ng://root@10.0.0.1:22", args[fromIdx+1])
}

// TestNixBuildEnv_RemoteModeUsesBuilderSSHOptions verifies NIX_SSHOPTS is
// generated from the pinned builder machine's SSH settings, not from
// whichever machine's goroutine executed the build.
func TestNixBuildEnv_RemoteModeUsesBuilderSSHOptions(t *testing.T) {
	t.Parallel()

	first := newRemoteMachine(t, "10.0.0.1")
	first.SSH.IdentityFile = "/tmp/test-key"
	second := newRemoteMachine(t, "10.0.0.2")
	inst := newRemoteInstallable(t, first, second)

	env, err := nixBuildEnv(inst, second)
	require.NoError(t, err)

	assert.Contains(t, strings.Join(env, " "), "IdentitiesOnly=yes")
}

// newLocalInstallable returns a local-mode installable with materialized nix
// defaults, matching config init.
func newLocalInstallable(t *testing.T) *installable.Installable {
	t.Helper()

	inst := &installable.Installable{}
	inst.Nix.BuildMode = nix.BuildModeLocal
	require.NoError(t, inst.Nix.Init(nil))

	inst.Machines = atomicorderedmap.New[string, *machine.Machine]()

	return inst
}

// TestNixBuildCommand_OutLinkReplacesNoLink swaps --no-link for
// --out-link while keeping the other default flags.
func TestNixBuildCommand_OutLinkReplacesNoLink(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable(t)

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, ".panix/my-flake/nixosConfigurations/server1")

	assert.Equal(t, -1, slices.Index(cmd, "--no-link"), "--no-link must be dropped: %v", cmd)

	outLinkIdx := slices.Index(cmd, "--out-link")
	require.NotEqual(t, -1, outLinkIdx, "command must contain --out-link: %v", cmd)
	assert.Equal(t, ".panix/my-flake/nixosConfigurations/server1", cmd[outLinkIdx+1])

	assert.Contains(t, cmd, "--print-out-paths")
	assert.Contains(t, cmd, "--keep-going")
}

// TestNixBuildCommand_NoOutLinkKeepsNoLink keeps the default flags
// unchanged when no outlink is requested.
func TestNixBuildCommand_NoOutLinkKeepsNoLink(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable(t)

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, "")

	assert.Contains(t, cmd, "--no-link")
	assert.Equal(t, -1, slices.Index(cmd, "--out-link"))
}

// TestCombineBuildFlags_CustomDefaultsWithoutNoLink handles custom default
// flags that contain no --no-link.
func TestCombineBuildFlags_CustomDefaultsWithoutNoLink(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable(t)
	inst.Nix.BuildDefaultFlags = []string{"--print-out-paths"}

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, "out/link")

	assert.Equal(t, []string{
		"nix", "build", "--extra-experimental-features", "nix-command flakes",
		"--print-out-paths", "--out-link", "out/link", "flake#attr",
	}, cmd)
}

// TestCombineBuildFlags_DoesNotMutateConfiguredFlags guards the caller's
// configured slice: combineBuildFlags must clone before rewriting, since the
// same config is used for repeated builds.
func TestCombineBuildFlags_DoesNotMutateConfiguredFlags(t *testing.T) {
	t.Parallel()

	cfg := &nix.NixConfig{BuildDefaultFlags: []string{"--no-link", "--print-out-paths", "--keep-going"}}
	before := slices.Clone(cfg.BuildDefaultFlags)

	got := combineBuildFlags(cfg, ".panix/link")

	assert.Equal(t, before, cfg.BuildDefaultFlags, "the configured slice must stay untouched")
	assert.Equal(t, []string{"--print-out-paths", "--keep-going", "--out-link", ".panix/link"}, got)
}

// TestNixBuildCommand_CommandOverride verifies issue #35: the build command can
// be delegated to a drop-in monitor without losing the flags panix depends on.
func TestNixBuildCommand_CommandOverride(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable(t)
	inst.Nix.BuildCommand = []string{"nom", "build"}

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, "")

	assert.Equal(t, []string{
		"nom", "build",
		"--extra-experimental-features", "nix-command flakes",
		"--no-link", "--print-out-paths", "--keep-going",
		"flake#attr",
	}, cmd)
}

// TestNixCopyBaseArgs_CommandOverride covers the copy counterpart.
func TestNixCopyBaseArgs_CommandOverride(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable(t)
	inst.Nix.CopyCommand = []string{"nom", "copy"}

	args, err := nixCopyBaseArgs(inst, "ssh-ng://root@10.0.0.2:22")
	require.NoError(t, err)

	assert.Equal(t, []string{
		"nom", "copy",
		"--extra-experimental-features", "nix-command flakes",
		"--to", "ssh-ng://root@10.0.0.2:22",
		"--no-check-sigs",
	}, args)
}

// The destination store is a configured flag value, so a replacement tool can
// place it where its own CLI expects, next to other operands.
func TestNixCopyBaseArgs_StoreURLFromDefaultFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		flags []string
		want  []string
	}{
		{
			name:  "reordered with an extra flag",
			flags: []string{"--no-check-sigs", "--to", "$PANIX_STORE_URL", "--keep-going"},
			want: []string{
				"nix", "copy",
				"--extra-experimental-features", "nix-command flakes",
				"--no-check-sigs", "--to", "ssh-ng://root@10.0.0.2:22", "--keep-going",
			},
		},
		{
			name:  "empty list passes no store at all",
			flags: []string{},
			want: []string{
				"nix", "copy",
				"--extra-experimental-features", "nix-command flakes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inst := newLocalInstallable(t)
			inst.Nix.CopyDefaultFlags = tt.flags

			args, err := nixCopyBaseArgs(inst, "ssh-ng://root@10.0.0.2:22")
			require.NoError(t, err)

			assert.Equal(t, tt.want, args)
		})
	}
}

// A flag list referencing a value the copy step does not supply fails instead of
// running a command with a half-expanded argument.
func TestNixCopyBaseArgs_UnavailableRuntimeVariableFails(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable(t)
	inst.Nix.CopyDefaultFlags = []string{"--to", "$PANIX_CLOSURE"}

	_, err := nixCopyBaseArgs(inst, "ssh-ng://root@10.0.0.2:22")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "copy_default_flags")
	assert.Contains(t, err.Error(), "PANIX_CLOSURE")
}

// A configured build command must be able to satisfy the store-path contract end
// to end: the executed binary runs through the real PTY transport and its last
// output line is the closure. The argv-only tests elsewhere cannot prove that.
func TestBuildInstallable_CommandOverrideEndToEnd(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "build-stub")

	const storePath = "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-stub"

	// Mimics a drop-in: prints progress noise, then the path last.
	script := "#!/bin/sh\necho 'building something'\necho '" + storePath + "'\n"
	//nolint:gosec // the stub must be executable for the PTY transport to run it
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	inst := newLocalInstallable(t)
	inst.Nix.BuildCommand = []string{binPath, "build"}

	mach := newLocalTransferMachine(t, true)
	leaf := &fleet.FleetLeaf{Flake: &flake.Flake{}, Installable: inst, Machine: mach}

	exc, _ := realExecutionerFor(t, mach)

	got, err := BuildInstallable(exc, leaf, []string{"flake#attr"}, "stub build", "")
	require.NoError(t, err)
	assert.Equal(t, storePath, got, "the last output line is the closure")
}

// The same stub without a store path as its last line must fail loudly, which is
// the contract documented for build_command.
func TestBuildInstallable_CommandOverrideWithoutStorePathFails(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "bad-stub")

	script := "#!/bin/sh\necho 'built fine'\necho 'done'\n"
	//nolint:gosec // the stub must be executable for the PTY transport to run it
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	inst := newLocalInstallable(t)
	inst.Nix.BuildCommand = []string{binPath, "build"}

	mach := newLocalTransferMachine(t, true)
	leaf := &fleet.FleetLeaf{Flake: &flake.Flake{}, Installable: inst, Machine: mach}

	exc, _ := realExecutionerFor(t, mach)

	_, err := BuildInstallable(exc, leaf, []string{"flake#attr"}, "stub build", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoBuildOutputs)
}

// The last line must be a full store path, not merely a string that starts like
// one: a banner, a TUI footer or a truncated capture would otherwise pass.
func TestIsStorePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"real path", "/nix/store/p0qzdv6bk2gxncwj99vqfaxvhg6fa7b4-h", true},
		{"path with name suffix", "/nix/store/8cmwhblx6ic1w6kg5khcw3gz8bcjnjsn-slowbuild", true},
		{"bare prefix", "/nix/store/", false},
		{"truncated hash", "/nix/store/p0qzdv6bk-ghc", false},
		{"bad hash chars", "/nix/store/eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee-x", false},
		{"tui footer", "Finished at 22:34:58 after 0s", false},
		{"ansi wrapped path", "\x1b[1m/nix/store/p0qzdv6bk2gxncwj99vqfaxvhg6fa7b4-h\x1b[0m", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isStorePath(tt.in))
		})
	}
}
