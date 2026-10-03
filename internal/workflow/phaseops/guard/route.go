package guard

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
)

// ShouldGuard is the routing predicate (spec 2, T3c): a deploy runs under the
// activation guard when
//
//   - the user tier is guarded (rollback: auto or magic; off/unset is legacy),
//   - the output type carries a supported tier (Preset.GuardTier != none),
//   - the mode mutates the target (NonMutatingModes, dry-activate included,
//     stay direct: nothing to protect or revert),
//   - and the preset owns a profile to transact on.
//
// The bootstrap path never reaches this predicate (it early-returns in the
// activate handler before activation runs).
func ShouldGuard(gate ConfirmationGate, tier installable.GuardTier, mode string, nonMutatingModes []string, profilePath string) bool {
	if !gate.IsGuarded() {
		return false
	}

	if tier == installable.GuardTierNone || tier == "" {
		return false
	}

	if slices.Contains(nonMutatingModes, mode) {
		return false
	}

	return profilePath != ""
}

// IsGuarded reports whether the user tier routes deploys through the guard.
// The literal values match internal/config/attributes.Rollback, so the handler
// converts the configured tier with a plain type conversion.
func (g ConfirmationGate) IsGuarded() bool {
	return g == GateAuto || g == GateMagic
}

// EffectiveGate resolves the confirmation gate the guardian runs with: the
// user's magic tier only applies where the transaction has a gate-worthy shape
// (composeStepLists reports eligibility); everything else guarded runs auto.
func EffectiveGate(userGate ConfirmationGate, eligible bool) ConfirmationGate {
	if userGate == GateMagic && eligible {
		return GateMagic
	}

	return GateAuto
}
