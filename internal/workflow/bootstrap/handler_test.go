package bootstrap

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pins the bootstrap order: kexec, hardware config, disko. Disko's build
// evaluates the operator's configuration, which may import the generated
// hardware config.
func TestRunPhase_KexecThenHardwareConfigThenDisko(t *testing.T) {
	// Not parallel: t.Setenv forbids it. Sandbox XDG_CACHE_HOME so the
	// dry-run build stays out of the real user cache.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	leaf := newDiskoLeaf(t, true)
	leaf.Machine.HardwareConfigPath = filepath.Join(t.TempDir(), "hardware-configuration.nix")
	leaf.Machine.Bootstrap.Kexec.Image = "https://example.com/kexec.tar.gz"
	leaf.Machine.MetaInspect.Store(&machine.MetaInspect{IsRoot: true, RequiresKexec: true})

	exc, phaseLog := testutil.NewDryRunExecutioner(t, leaf.Machine, phase.Bootstrap)

	require.NoError(t, Handler{}.RunPhase(exc, leaf))

	lines := nonEmptyCommandLines(t, phaseLog)

	kexecIdx := indexOfCommandContaining(t, lines, "/tmp/kexec/kexec/run")
	configIdx := indexOfCommandContaining(t, lines, "nixos-generate-config --show-hardware-config --no-filesystems")
	diskoIdx := indexOfCommandContaining(t, lines, "disko_BUILD_OUTPUT_PATH_PLACEHOLDER")

	assert.Less(t, kexecIdx, configIdx, "hardware config must be generated after kexec boots the installer")
	assert.Less(t, configIdx, diskoIdx, "hardware config must exist before the disko build evaluates the configuration")
}

// nonEmptyCommandLines drops argv-less entries recorded by ExecFn wait loops.
func nonEmptyCommandLines(t *testing.T, phaseLog *phaselogs.PhaseLog) []string {
	t.Helper()

	var lines []string

	for _, line := range testutil.CommandLines(t, phaseLog) {
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

func indexOfCommandContaining(t *testing.T, lines []string, substr string) int {
	t.Helper()

	for i, line := range lines {
		if strings.Contains(line, substr) {
			return i
		}
	}

	require.FailNowf(t, "command not recorded", "no command containing %q in %v", substr, lines)

	return -1
}

// hasCommandContaining reports whether any recorded line contains substr.
func hasCommandContaining(lines []string, substr string) bool {
	for _, line := range lines {
		if strings.Contains(line, substr) {
			return true
		}
	}

	return false
}

// newMetaInspect returns the MetaInspect that atomicpointer.New creates and
// stores: the unprobed zero value (inspect skipped / nothing probed yet).
// The sibling unprobed state is nil, the JSON "null" round-trip state.
func newMetaInspect() *machine.MetaInspect {
	return atomicpointer.New[machine.MetaInspect]().Load()
}

// Pins the bootstrap order for the nix-install mode: fetch installer, mark
// it executable, run installer, verify. The verify probe must run after the
// installer, it refreshes MetaInspect.NixAvailable for the later phases.
func TestRunPhase_NixInstall_FetchInstallerThenRunInstallerThenVerify(t *testing.T) {
	// Not parallel: t.Setenv forbids it. Sandbox XDG_CACHE_HOME so the
	// dry-run build stays out of the real user cache.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	leaf := newDiskoLeaf(t, true)
	leaf.Installable.Preset.Bootstrap = installable.BootstrapNixInstall
	leaf.Machine.MetaInspect.Store(&machine.MetaInspect{IsRoot: true})

	exc, phaseLog := testutil.NewDryRunExecutioner(t, leaf.Machine, phase.Bootstrap)

	require.NoError(t, Handler{}.RunPhase(exc, leaf))

	lines := nonEmptyCommandLines(t, phaseLog)

	fetchIdx := indexOfCommandContaining(t, lines, "curl --fail")
	chmodIdx := indexOfCommandContaining(t, lines, "chmod +x /tmp/panix-nix-installer")
	runIdx := indexOfCommandContaining(t, lines, "/tmp/panix-nix-installer install --no-confirm")
	verifyIdx := indexOfCommandContaining(t, lines, "nix --version")

	assert.Less(t, fetchIdx, chmodIdx, "the installer must be fetched before it is marked executable")
	assert.Less(t, chmodIdx, runIdx, "the installer must be executable before it is run")
	assert.Less(t, runIdx, verifyIdx, "nix must be verified after the installer ran")

	assert.True(t, leaf.Machine.MetaInspect.Load().NixAvailable,
		"verification must refresh MetaInspect.NixAvailable")
}

// ShouldSkip matrix: the NixOS mode keys on Bootstrapped, the nix-install
// mode on NixAvailable, custom output types always skip; ForceBootstrap
// forces the phase in every mode. An unprobed MetaInspect (nil, or the zero
// value atomicpointer.New leaves) must never skip the bootstrap.
func TestShouldSkip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mode  installable.BootstrapMode
		mi    *machine.MetaInspect
		force bool
		want  bool
	}{
		{"nixos: skip when bootstrapped", installable.BootstrapNixOS, &machine.MetaInspect{Bootstrapped: true}, false, true},
		{"nixos: run when not bootstrapped", installable.BootstrapNixOS, &machine.MetaInspect{Bootstrapped: false}, false, false},
		{"nixos: run when MetaInspect is nil", installable.BootstrapNixOS, nil, false, false},
		{"nixos: run when MetaInspect is the atomicpointer.New zero value", installable.BootstrapNixOS, newMetaInspect(), false, false},
		{"nixos: force runs when bootstrapped", installable.BootstrapNixOS, &machine.MetaInspect{Bootstrapped: true}, true, false},
		{"nix-install: skip when nix available", installable.BootstrapNixInstall, &machine.MetaInspect{NixAvailable: true}, false, true},
		{"nix-install: run when nix missing", installable.BootstrapNixInstall, &machine.MetaInspect{NixAvailable: false}, false, false},
		{"nix-install: run when MetaInspect is nil", installable.BootstrapNixInstall, nil, false, false},
		{"nix-install: run when MetaInspect is the atomicpointer.New zero value", installable.BootstrapNixInstall, newMetaInspect(), false, false},
		{"nix-install: force runs when nix available", installable.BootstrapNixInstall, &machine.MetaInspect{NixAvailable: true}, true, false},
		{"none: always skip", installable.BootstrapNone, &machine.MetaInspect{NixAvailable: false}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			leaf := newDiskoLeaf(t, true)
			leaf.Installable.Preset.Bootstrap = tt.mode
			leaf.Machine.Bootstrap.ForceBootstrap = tt.force
			leaf.Machine.MetaInspect.Store(tt.mi)

			assert.Equal(t, tt.want, Handler{}.ShouldSkip(leaf))
		})
	}
}

// Install-condition matrix for the nix-install branch: the install step runs
// only when nix is missing (or forced) and bootstrap.disable_nix_install does
// not opt out. The opt-out must hold even when the inspect gate was skipped,
// and the user-declared post_bootstrap_hooks always run.
func TestRunPhase_NixInstall_InstallCondition(t *testing.T) {
	tests := []struct {
		name              string
		nixAvailable      bool
		disableNixInstall bool
		force             bool
		wantInstall       bool
	}{
		{"nix missing: fetch, run, verify", false, false, false, true},
		{"nix present: install step skipped", true, false, false, false},
		{"force re-installs when nix is present", true, false, true, true},
		{"disable_nix_install with nix missing: no fetch or run argv", false, true, false, false},
		{"disable_nix_install wins over force", true, true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: t.Setenv forbids it. Sandbox XDG_CACHE_HOME
			// so the dry-run build stays out of the real user cache.
			t.Setenv("XDG_CACHE_HOME", t.TempDir())

			leaf := newDiskoLeaf(t, true)
			leaf.Installable.Preset.Bootstrap = installable.BootstrapNixInstall
			leaf.Machine.Bootstrap.DisableNixInstall = tt.disableNixInstall
			leaf.Machine.Bootstrap.ForceBootstrap = tt.force
			leaf.Machine.Bootstrap.PostBootstrapHooks = []attributes.PostBootstrapHookCommand{"echo hook-ran"}
			leaf.Machine.MetaInspect.Store(&machine.MetaInspect{IsRoot: true, NixAvailable: tt.nixAvailable})

			exc, phaseLog := testutil.NewDryRunExecutioner(t, leaf.Machine, phase.Bootstrap)

			require.NoError(t, Handler{}.RunPhase(exc, leaf))

			lines := nonEmptyCommandLines(t, phaseLog)

			// Hooks are user-declared and run in every case.
			assert.True(t, hasCommandContaining(lines, "echo hook-ran"),
				"post_bootstrap_hooks must run regardless of the install step")

			installSteps := []string{
				"curl --fail",
				"chmod +x /tmp/panix-nix-installer",
				"/tmp/panix-nix-installer install --no-confirm",
				"nix --version",
			}
			for _, step := range installSteps {
				assert.Equal(t, tt.wantInstall, hasCommandContaining(lines, step),
					"install step %q presence must match wantInstall", step)
			}
		})
	}
}

// With MetaInspect nil or holding only the zero value atomicpointer.New
// leaves (nothing probed yet, e.g. the Inspect phase was skipped) the
// bootstrap must perform the full install: fetch, run and the verification
// probe, which refreshes MetaInspect.NixAvailable on top of the unprobed
// base.
func TestRunNixInstallBootstrap_NilMetaInspectPerformsInstall(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*machine.Machine)
	}{
		{
			name: "MetaInspect nil (inspect skipped): full install chain",
			setup: func(mach *machine.Machine) {
				mach.MetaInspect.Store(nil)
			},
		},
		{
			name: "MetaInspect zero value from atomicpointer.New (nothing probed yet): full install chain",
			setup: func(mach *machine.Machine) {
				mach.MetaInspect.Store(newMetaInspect())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel: t.Setenv forbids it. Sandbox XDG_CACHE_HOME so
			// the dry-run build stays out of the real user cache.
			t.Setenv("XDG_CACHE_HOME", t.TempDir())

			leaf := newDiskoLeaf(t, true)
			leaf.Installable.Preset.Bootstrap = installable.BootstrapNixInstall
			tt.setup(leaf.Machine)

			exc, phaseLog := testutil.NewDryRunExecutioner(t, leaf.Machine, phase.Bootstrap)

			require.NoError(t, runNixInstallBootstrap(exc, leaf.Machine))

			lines := nonEmptyCommandLines(t, phaseLog)
			assert.True(t, hasCommandContaining(lines, "curl --fail"), "the installer must be fetched")
			assert.True(t, hasCommandContaining(lines, "chmod +x /tmp/panix-nix-installer"), "the staged installer must be marked executable")
			assert.True(t, hasCommandContaining(lines, "/tmp/panix-nix-installer install --no-confirm"), "the installer must be run")
			assert.True(t, hasCommandContaining(lines, "nix --version"), "the verification probe must run")

			assert.True(t, leaf.Machine.MetaInspect.Load().NixAvailable,
				"verification must refresh MetaInspect.NixAvailable")
		})
	}
}
