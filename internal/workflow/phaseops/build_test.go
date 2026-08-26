package phaseops

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/nix"
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
// machines in declaration order.
func newRemoteInstallable(machines ...*machine.Machine) *installable.Installable {
	inst := &installable.Installable{}
	inst.Nix.BuildMode = nix.BuildModeRemote
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
	inst := newRemoteInstallable(first, second)

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

	inst := newRemoteInstallable()

	assert.PanicsWithValue(
		t,
		"internal error: remote-mode installable has no machines (validation should have rejected it)",
		func() { remoteBuilderSSH(inst) },
	)
}

func TestNixBuildCommand_LocalModeHasNoStoreFlag(t *testing.T) {
	t.Parallel()

	m := newRemoteMachine(t, "10.0.0.1")
	inst := newRemoteInstallable(m)
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
	inst := newRemoteInstallable(first, second)

	args := nixCopyBaseArgs(inst, "ssh-ng://root@10.0.0.2:22")

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
	inst := newRemoteInstallable(first, second)

	env, err := nixBuildEnv(inst, second)
	require.NoError(t, err)

	assert.Contains(t, strings.Join(env, " "), "IdentitiesOnly=yes")
}

// newLocalInstallable returns a local-mode installable with default nix config.
func newLocalInstallable() *installable.Installable {
	inst := &installable.Installable{}
	inst.Nix.BuildMode = nix.BuildModeLocal
	inst.Machines = atomicorderedmap.New[string, *machine.Machine]()

	return inst
}

// TestNixBuildCommand_OutLinkReplacesNoLink swaps --no-link for
// --out-link while keeping the other default flags.
func TestNixBuildCommand_OutLinkReplacesNoLink(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable()

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

	inst := newLocalInstallable()

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, "")

	assert.Contains(t, cmd, "--no-link")
	assert.Equal(t, -1, slices.Index(cmd, "--out-link"))
}

// TestCombineBuildFlags_CustomDefaultsWithoutNoLink handles custom default
// flags that contain no --no-link.
func TestCombineBuildFlags_CustomDefaultsWithoutNoLink(t *testing.T) {
	t.Parallel()

	inst := newLocalInstallable()
	inst.Nix.BuildDefaultFlags = []string{"--print-out-paths"}

	cmd := nixBuildCommand(inst, []string{"flake#attr"}, "out/link")

	assert.Equal(t, []string{
		"nix", "--extra-experimental-features", "nix-command flakes", "build",
		"--print-out-paths", "--out-link", "out/link", "flake#attr",
	}, cmd)
}

// TestCombineBuildFlags_DoesNotMutateBuiltInDefaults guards the shared default
// flag slice against mutation.
func TestCombineBuildFlags_DoesNotMutateBuiltInDefaults(t *testing.T) {
	t.Parallel()

	before := slices.Clone(nix.DefaultBuildFlags)

	combineBuildFlags(&nix.NixConfig{}, ".panix/link")

	assert.Equal(t, before, nix.DefaultBuildFlags)
	assert.Contains(t, nix.DefaultBuildFlags, "--no-link")
}
