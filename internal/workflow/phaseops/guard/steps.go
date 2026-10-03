package guard

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
)

// Step-list composition (spec 4.2): panix composes the activation child, the
// commit transaction and the revert transaction for one output type and mode;
// the guardian stays agnostic and executes the lists it receives (spec 6.5).
//
// The output type's preset (internal/config/tree/installable/presets.go) is
// the single source of truth for the per-type semantics: the tier class and
// both script paths (ActivationPath, GuardCommitScript) come from the same
// table row that declares every other per-type behavior.

// ComposedSteps bundles one transaction's panix-composed step lists with the
// invariant target and the magic-gate eligibility the deployer needs to spawn
// the guardian (spec 6.5).
type ComposedSteps struct {
	Activation      StepList
	Commit          StepList
	Revert          StepList
	InvariantTarget string
	GateEligible    bool
}

// composeStepLists composes the transaction steps for one output type and
// mode (spec 4.2). profile, nixEnv, old and new are absolute paths resolved
// at pre-start.
//
// Gate eligibility (spec 2): magic gates when the transaction has something
// to confirm - a non-empty commit on full/standard tiers - or unconditionally
// on self-setting tiers (no commit, but the gate still decides commit-keep
// vs revert). Minimal never gates.
func composeStepLists(preset installable.Preset, mode, profile, nixEnv, old, new string) ComposedSteps {
	tier := preset.GuardTierValue()
	activation := activationStepList(preset, tier, mode, new)
	commit := commitStepList(preset, tier, mode, profile, nixEnv, new)
	revert := revertStepList(preset, tier, mode, profile, nixEnv, old)

	return ComposedSteps{
		Activation:      activation,
		Commit:          commit,
		Revert:          revert,
		InvariantTarget: invariantTargetFor(tier, mode, old, new),
		GateEligible:    gateEligibleFor(tier, commit),
	}
}

// activationStepList composes the activation child (spec 4.1 step 9, 6.5).
// The full tier runs switch-to-configuration with the mode argument: switch
// and test both activate in test mode (the commit applies the profile), boot
// activates in boot mode. Every other tier runs its activation script
// without a mode.
func activationStepList(preset installable.Preset, tier installable.GuardTier, mode, new string) StepList {
	if tier == installable.GuardTierNone || preset.ActivationPath == "" {
		return nil
	}

	if tier == installable.GuardTierFull {
		arg := "test"
		if mode == "boot" {
			arg = "boot"
		}

		return StepList{{new + "/" + preset.ActivationPath, arg}}
	}

	return StepList{{new + "/" + preset.ActivationPath}}
}

// commitStepList composes the commit transaction (spec 4.2). --set is
// idempotent, so re-executed lists converge.
func commitStepList(preset installable.Preset, tier installable.GuardTier, mode, profile, nixEnv, new string) StepList {
	switch {
	case tier == installable.GuardTierNone, tier == installable.GuardTierMinimal, tier == installable.GuardTierSelfSetting:
		return nil
	case tier == installable.GuardTierFull && mode == "test":
		return nil // profile untouched per ProfileSkipModes
	case tier == installable.GuardTierFull && mode == "boot":
		return nil // profile set at pre-start; boot ran during activation
	case tier == installable.GuardTierFull:
		// nixos switch: set the profile, then finalize the commit with the
		// activation path in boot mode.
		return StepList{
			{nixEnv, "-p", profile, "--set", new},
			{new + "/" + preset.ActivationPath, "boot"},
		}
	default:
		// standard tiers: --set, then the commit script when the type has one
		// (system-manager's register-profile also registers its own gcroot,
		// verified V1); bare --set otherwise (darwin: activate is
		// profile-agnostic, verified V2).
		if preset.GuardCommitScript != "" {
			return StepList{{new + "/" + preset.GuardCommitScript}}
		}

		return StepList{{nixEnv, "-p", profile, "--set", new}}
	}
}

// revertStepList composes the revert transaction (spec 4.2): where a
// bootloader exists, revert must rewrite the boot default (never test on
// switch deploys); test-mode deploys restore by re-running OLD's activation
// in test mode; boot mode is profile-first, so the revert restores the
// profile entry before re-running OLD's boot activation (the guardian's
// runtime restore covers partial commits on top; both layers stay).
func revertStepList(preset installable.Preset, tier installable.GuardTier, mode, profile, nixEnv, old string) StepList {
	switch {
	case tier == installable.GuardTierNone, tier == installable.GuardTierMinimal:
		return nil
	case tier == installable.GuardTierFull && mode == "test":
		return StepList{{old + "/" + preset.ActivationPath, "test"}}
	case tier == installable.GuardTierFull && mode == "boot":
		return StepList{
			{nixEnv, "-p", profile, "--set", old},
			{old + "/" + preset.ActivationPath, "boot"},
		}
	case tier == installable.GuardTierFull:
		return StepList{{old + "/" + preset.ActivationPath, "switch"}}
	default:
		// standard tiers: the commit script first when present (system-manager:
		// register restores the profile and its gcroot, activate diffs the
		// state back, verified V1), then OLD's activation; self-setting:
		// OLD's activate re-flips the user profile.
		var steps StepList
		if preset.GuardCommitScript != "" {
			steps = append(steps, []string{old + "/" + preset.GuardCommitScript})
		}

		return append(steps, []string{old + "/" + preset.ActivationPath})
	}
}

// invariantTargetFor returns the store path the profile must resolve to after
// the transaction's effects (spec 6.5): switch/boot activate NEW, test must
// leave OLD untouched, self-setting's activation advances the profile to NEW;
// minimal skips the check.
func invariantTargetFor(tier installable.GuardTier, mode, old, new string) string {
	if tier == installable.GuardTierNone || tier == installable.GuardTierMinimal {
		return ""
	}

	if tier == installable.GuardTierFull && mode == "test" {
		return old
	}

	return new
}

// gateEligibleFor reports whether the magic gate is meaningful for this
// transaction (spec 2): full/standard tiers with a non-empty commit, or
// self-setting (the gate decides keep-vs-revert without a commit step).
func gateEligibleFor(tier installable.GuardTier, commit StepList) bool {
	if tier == installable.GuardTierSelfSetting {
		return true
	}

	if tier != installable.GuardTierFull && tier != installable.GuardTierStandard {
		return false
	}

	return len(commit) > 0
}

// BootModeGateIneligible reports statically whether a transaction for the
// preset and mode is always gate-ineligible (spec 2, 5): the full tier's
// boot-mode commit is empty by design (the profile set moves to pre-start and
// the bootloader install is part of the activation), so the magic tier
// degrades to auto (EffectiveGate) and the panix-side remote health_checks
// would never run. composeStepLists derives the same verdict at runtime from
// the composed commit list; this predicate reads the same composition so
// config-time validation cannot drift from the runtime gate. Boot is the one
// statically resolvable mode where eligibility is always false; test mode is
// out of scope here (its emptiness is a profile-skip, not a boot-mode rule).
func BootModeGateIneligible(preset installable.Preset, mode string) bool {
	if mode != "boot" {
		return false
	}

	tier := preset.GuardTierValue()
	if tier != installable.GuardTierFull {
		return false
	}

	return len(commitStepList(preset, tier, mode, "", "", "")) == 0
}

// gcRootTarget returns the gc-root symlink target for the transaction (the
// authoritative table in spec 4.3): profile-last tiers root NEW (the profile
// protects OLD); boot mode and self-setting root OLD (the profile was or will
// be re-pointed, demoting OLD); minimal roots nothing.
func gcRootTarget(tier installable.GuardTier, mode, old, new string) string {
	if tier == installable.GuardTierNone || tier == installable.GuardTierMinimal {
		return ""
	}

	if tier == installable.GuardTierFull && mode == "boot" {
		return old
	}

	if tier == installable.GuardTierSelfSetting {
		return old
	}

	return new
}
