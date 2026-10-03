package guard

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/guard"

	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exitErr synthesizes an exec error with the given exit code (the lock verb's
// held-lock exit is 3).
func exitErr(t *testing.T, code int) error {
	t.Helper()

	err := exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run()
	require.Error(t, err)

	return err
}

// TestLegacySweepPlan pins the pre-start legacy-slot decision table (spec 10.1
// coexistence): absent slots are skipped, free slots are removed, live legacy
// transactions fail fast, and unprobeable states fail closed.
func TestLegacySweepPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		facts legacyFacts
		want  legacyPlan
	}{
		{
			name:  "legacy absent: skip",
			facts: legacyFacts{},
			want:  legacySkip,
		},
		{
			name:  "legacy inspect failed: fail closed",
			facts: legacyFacts{dirExists: true, inspectErr: exitErr(t, guard.InspectExitUnparseable)},
			want:  legacyFailClosed,
		},
		{
			name:  "legacy log missing: remove the tree (no log inode, no lock can exist)",
			facts: legacyFacts{dirExists: true, logMissing: true},
			want:  legacyRemove,
		},
		{
			name:  "legacy slot free: remove the tree",
			facts: legacyFacts{dirExists: true},
			want:  legacyRemove,
		},
		{
			name:  "legacy status but locked: fail fast (the lock state stays valid for @PG1 logs)",
			facts: legacyFacts{dirExists: true, locked: true},
			want:  legacyLive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, legacySweepPlan(tt.facts))
		})
	}
}

// TestLegacyLiveMessage pins the fail-fast wording: the legacy-live failure
// must carry the live-deploy indicator (the concurrent-deploy e2e contract)
// and name the legacy transaction.
func TestLegacyLiveMessage(t *testing.T) {
	t.Parallel()

	message := legacyLiveMessage("/run/panix-guard/legacy-slot")

	assert.True(t, strings.HasPrefix(message, "live deploy"), "must carry the live-deploy indicator: %s", message)
	assert.Contains(t, message, "legacy")
	assert.Contains(t, message, "/run/panix-guard/legacy-slot")
}

// TestSweepLegacySlotDryRun pins the preview contract: dry-run assumes a clean
// target and issues no legacy commands, exactly like the v2 lock probe.
func TestSweepLegacySlotDryRun(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, false, "auto")
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	require.NoError(t, sweepLegacySlot(executionerSurface{ex: exc}, mach, guardPreset(t), "",
		"/run/panix-guard/v2/slot", "/run/panix-guard/legacy-slot"))
	assert.Empty(t, testutil.CommandLines(t, phaseLog), "the legacy sweep must stay silent in dry-run")
}

// TestSweepLegacySlot_InspectLockField pins the exec-path sweep decisions
// through the inspect verdict's lock field (spec 9.5: the lock state stays
// valid even for a legacy @PG1 log): a legacy-status-but-locked slot fails
// fast like a live v2 slot, a free legacy slot is swept away, and the probe
// itself stays an inspect one-shot against the legacy directory.
func TestSweepLegacySlot_InspectLockField(t *testing.T) {
	t.Parallel()

	t.Run("legacy status but locked fails fast with the live-deploy contract", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.script("legacy guard slot probe", fakeStepOutcome{output: "yes\n"})
		surface.script("legacy guard slot lock probe",
			fakeStepOutcome{output: `{"status":"legacy","terminal":false,"lock":true}` + "\n"})

		err := sweepLegacySlot(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
			"/run/panix-guard/v2/slot", "/run/panix-guard/legacy-slot")
		require.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "live deploy"), "the concurrent-deploy contract: %s", err.Error())
		assert.Contains(t, err.Error(), "legacy")

		for _, call := range surface.stepCalls() {
			assert.NotContains(t, call, "legacy guard slot cleanup", "a live legacy slot is never mutated")
		}
	})

	t.Run("legacy status and free is removed", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.script("legacy guard slot probe", fakeStepOutcome{output: "yes\n"})
		surface.script("legacy guard slot lock probe",
			fakeStepOutcome{output: `{"status":"legacy","terminal":false,"lock":false}` + "\n"})
		surface.script("legacy guard slot cleanup", fakeStepOutcome{})

		err := sweepLegacySlot(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
			"/run/panix-guard/v2/slot", "/run/panix-guard/legacy-slot")
		require.NoError(t, err)

		inspect := surface.stepCalls()[callIndex(surface.stepCalls(), "legacy guard slot lock probe")]
		assert.Contains(t, inspect, "'--dir' '/run/panix-guard/legacy-slot'",
			"the lock state comes from the v2 binary's inspect against the legacy directory")
	})
}
