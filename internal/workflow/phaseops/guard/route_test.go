package guard

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/stretchr/testify/assert"
)

// TestShouldGuard pins the routing predicate (tier x mode x profile matrix,
// spec 2): guarded only when the user tier is auto/magic, the output type's
// guard tier is supported, the mode mutates the target, and a profile exists.
func TestShouldGuard(t *testing.T) {
	t.Parallel()

	nixosNonMutating := []string{"dry-activate"}

	tests := []struct {
		name        string
		gate        ConfirmationGate
		tier        installable.GuardTier
		mode        string
		profilePath string
		want        bool
	}{
		{name: "auto full switch guards", gate: GateAuto, tier: installable.GuardTierFull, mode: "switch", profilePath: testProfile, want: true},
		{name: "magic full boot guards", gate: GateMagic, tier: installable.GuardTierFull, mode: "boot", profilePath: testProfile, want: true},
		{name: "magic full test guards", gate: GateMagic, tier: installable.GuardTierFull, mode: "test", profilePath: testProfile, want: true},
		{name: "auto standard guards", gate: GateAuto, tier: installable.GuardTierStandard, mode: "switch", profilePath: testProfile, want: true},
		{name: "magic self-setting guards", gate: GateMagic, tier: installable.GuardTierSelfSetting, mode: "switch", profilePath: testProfile, want: true},
		{name: "auto minimal guards", gate: GateAuto, tier: installable.GuardTierMinimal, mode: "switch", profilePath: testProfile, want: true},

		{name: "off is legacy", gate: GateOff, tier: installable.GuardTierFull, mode: "switch", profilePath: testProfile, want: false},
		{name: "unset gate is legacy", gate: "", tier: installable.GuardTierFull, mode: "switch", profilePath: testProfile, want: false},
		{name: "none tier is legacy (packages)", gate: GateAuto, tier: installable.GuardTierNone, mode: "switch", profilePath: "", want: false},
		{name: "unknown tier is legacy", gate: GateAuto, tier: "", mode: "switch", profilePath: testProfile, want: false},
		{name: "non-mutating mode is legacy", gate: GateAuto, tier: installable.GuardTierFull, mode: "dry-activate", profilePath: testProfile, want: false},
		{name: "no profile is legacy", gate: GateAuto, tier: installable.GuardTierFull, mode: "switch", profilePath: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ShouldGuard(tt.gate, tt.tier, tt.mode, nixosNonMutating, tt.profilePath))
		})
	}
}

// TestEffectiveGate pins the magic-gate resolution: magic survives only where
// the transaction shape is gate-worthy.
func TestEffectiveGate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, GateMagic, EffectiveGate(GateMagic, true))
	assert.Equal(t, GateAuto, EffectiveGate(GateMagic, false))
	assert.Equal(t, GateAuto, EffectiveGate(GateAuto, true))
	assert.Equal(t, GateAuto, EffectiveGate(GateOff, true))
}

// TestConfirmationGateIsGuarded pins the gate conversion from the configured
// rollback tier literals.
func TestConfirmationGateIsGuarded(t *testing.T) {
	t.Parallel()

	assert.False(t, GateOff.IsGuarded())
	assert.True(t, GateAuto.IsGuarded())
	assert.True(t, GateMagic.IsGuarded())
	assert.False(t, ConfirmationGate("").IsGuarded())
}
