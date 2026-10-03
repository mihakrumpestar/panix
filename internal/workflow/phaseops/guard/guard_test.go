package guard

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/guard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSlotDir pins the slot derivation (spec 10.1): deterministic slug + hash,
// same-basename profiles stay distinct, darwin vs linux roots split, and the
// v2 layout component rides on every root.
func TestSlotDir(t *testing.T) {
	t.Parallel()

	systemProfile := "/nix/var/nix/profiles/system"

	t.Run("deterministic", func(t *testing.T) {
		t.Parallel()

		first := SlotDir(systemProfile, true, false)
		second := SlotDir(systemProfile, true, false)

		assert.Equal(t, first, second)
		assert.Regexp(t, `^/run/panix-guard/v2/nix-var-nix-profiles-system-[0-9a-f]{8}$`, first)
	})

	t.Run("same basename stays distinct", func(t *testing.T) {
		t.Parallel()

		first := SlotDir("/nix/var/nix/profiles/system", true, false)
		second := SlotDir("/mnt/other/system", true, false)

		assert.NotEqual(t, first, second)
	})

	t.Run("darwin system slots live in /var/root", func(t *testing.T) {
		t.Parallel()

		assert.True(t, strings.HasPrefix(SlotDir(systemProfile, true, true), "/var/root/panix-guard/v2/"))
	})

	t.Run("user-level slots live in the state dir regardless of platform", func(t *testing.T) {
		t.Parallel()

		hm := "~/.local/state/nix/profiles/home-manager"

		assert.True(t, strings.HasPrefix(SlotDir(hm, false, false), "~/.local/state/panix-guard/v2/"))
		assert.True(t, strings.HasPrefix(SlotDir(hm, false, true), "~/.local/state/panix-guard/v2/"))
		// The tilde slug keeps no leading dash.
		assert.Regexp(t, `^~/.local/state/panix-guard/v2/local-state-nix-profiles-home-manager-[0-9a-f]{8}$`, SlotDir(hm, false, false))
	})

	t.Run("legacy slot path drops the v2 component", func(t *testing.T) {
		t.Parallel()

		v2 := SlotDir(systemProfile, true, false)
		legacy := LegacySlotDir(v2)

		assert.True(t, strings.HasPrefix(legacy, "/run/panix-guard/"))
		assert.NotContains(t, legacy, "/v2", "legacy path must not carry the v2 component: %s", legacy)
		assert.Equal(t, filepath.Base(v2), filepath.Base(legacy), "the slot name must survive the layout downgrade")

		nonV2 := "/run/panix-guard/plain-slot"
		assert.Equal(t, nonV2, LegacySlotDir(nonV2), "a non-v2 path maps to itself")
	})
}

// TestArgvSnapshots pins the guardian argv contracts (spec 6.5, 8, 9.1, 9.3).
func TestArgvSnapshots(t *testing.T) {
	t.Parallel()

	t.Run("spawn", func(t *testing.T) {
		t.Parallel()

		got := SpawnArgv(
			"/run/panix-guard/slot/panix-guard", "/run/panix-guard/slot", "key-1",
			testProfile, testNixEnv, testNixStore, testNew, 7, "switch", installable.GuardTierFull, GateMagic,
			15*time.Minute, time.Minute, true, []string{"systemctl is-system-running"},
			StepList{{testNew + "/bin/switch-to-configuration", "test"}},
			StepList{{testNixEnv, "-p", testProfile, "--set", testNew}},
			StepList{{testOld + "/bin/switch-to-configuration", "switch"}},
			testNew, testNew, "", true,
		)

		assert.Equal(t, []string{
			"/run/panix-guard/slot/panix-guard", "start",
			"--dir", "/run/panix-guard/slot",
			"--key", "key-1",
			"--profile", testProfile,
			"--nix-env", testNixEnv,
			"--new", testNew,
			"--gen", "7",
			"--mode", "switch",
			"--tier", "full",
			"--confirmation", "magic",
			"--activation-timeout", "15m0s",
			"--confirm-timeout", "1m0s",
			"--reboot-on-revert-failure=true",
			"--health-checks-local", `["systemctl is-system-running"]`,
			"--builtin-unit-check=true",
			"--activation-argv", `[["/nix/store/new/bin/switch-to-configuration","test"]]`,
			"--commit-argv", `[["/run/current-system/sw/bin/nix-env","-p","/nix/var/nix/profiles/system","--set","/nix/store/new"]]`,
			"--revert-argv", `[["/nix/store/old/bin/switch-to-configuration","switch"]]`,
			"--invariant-target", testNew,
			"--sweep=true",
			"--gc-root-target", testNew,
			"--nix-store", testNixStore,
		}, got)

		assert.NotContains(t, got, "--old", "start captures OLD post-converge; the flag left the spawn argv")
	})

	t.Run("spawn boot mode carries the pre-start set", func(t *testing.T) {
		t.Parallel()

		got := SpawnArgv(
			"panix-guard", "/slot", "k",
			testProfile, testNixEnv, testNixStore, testNew, 7, "boot", installable.GuardTierFull, GateAuto,
			15*time.Minute, time.Minute, false, nil,
			StepList{{testNew + "/bin/switch-to-configuration", "boot"}},
			nil,
			StepList{{testNixEnv, "-p", testProfile, "--set", testOld}, {testOld + "/bin/switch-to-configuration", "boot"}},
			testNew, testOld, testNew, true,
		)

		assert.Contains(t, got, "--boot-set", testNew, "boot mode wires the pre-start profile set")
		assert.Equal(t, testOld, got[slices.Index(got, "--gc-root-target")+1], "boot mode roots OLD")
		assert.Equal(t, "[]", got[slices.Index(got, "--commit-argv")+1], "boot mode has no commit step")
	})

	t.Run("spawn skips the absent mutation targets", func(t *testing.T) {
		t.Parallel()

		got := SpawnArgv(
			"panix-guard", "/slot", "k",
			testProfile, testNixEnv, testNixStore, testNew, 7, "switch", installable.GuardTierMinimal, GateAuto,
			time.Minute, time.Minute, false, nil,
			StepList{{testNew + "/activate"}}, nil, nil, "", "", "", true,
		)

		assert.NotContains(t, got, "--gc-root-target", "an empty gc-root target skips the flag")
		assert.NotContains(t, got, "--boot-set", "the boot set rides only boot mode")
		assert.Contains(t, got, "--sweep=true")
		assert.Contains(t, got, "--nix-store", testNixStore)
	})

	t.Run("inspect", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t,
			[]string{"panix-guard", "inspect", "--dir", "/slot"},
			InspectArgv("panix-guard", "/slot"))
	})

	t.Run("inspect probe wrapper demotes only the absent outcome", func(t *testing.T) {
		t.Parallel()

		argv := InspectProbeArgv("panix-guard", "/slot")
		require.Len(t, argv, 3)
		assert.Equal(t, "sh", argv[0])
		assert.Equal(t, "-c", argv[1])

		script := argv[2]

		assert.Contains(t, script, `'panix-guard' 'inspect' '--dir' '/slot'`, "the one-shot stays embedded verbatim")
		assert.Contains(t, script, "-eq "+strconv.Itoa(guard.InspectExitNoSlot), "only the expected-clean exit demotes")
		assert.Contains(t, script, "printf absent; exit 0", "the absent outcome folds into a zero exit and reports on stdout")
		assert.Contains(t, script, `exit "$rc"`, "every other exit passes through, so genuine failures stay loud")
	})

	t.Run("converge", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t,
			[]string{"panix-guard", "converge", "--dir", "/slot", "--profile", testProfile, "--nix-env", testNixEnv, "--truncate"},
			ConvergeArgv("panix-guard", "/slot", testProfile, testNixEnv, true))
		assert.Equal(t,
			[]string{"panix-guard", "converge", "--dir", "/slot", "--profile", testProfile, "--nix-env", testNixEnv},
			ConvergeArgv("panix-guard", "/slot", testProfile, testNixEnv, false), "inline post-mortem keeps the log for reporting")

		assert.NotContains(t, ConvergeArgv("panix-guard", "/slot", "", "", false), "--profile", "empty hints are omitted")
		assert.NotContains(t, ConvergeArgv("panix-guard", "/slot", "", "", false), "--nix-env")
	})

	t.Run("ctl", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t,
			[]string{"panix-guard", "ctl", "--dir", "/slot", "--key", "k", "--wait", "5s", "confirm"},
			CtlArgv("panix-guard", "/slot", "k", "confirm", 5*time.Second, ""))
	})
}
