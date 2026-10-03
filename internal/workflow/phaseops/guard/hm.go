package guard

import (
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/pkg/errors"
)

// Home-manager per-deploy composition (spec 4.2 tier table): HM >= 25.11
// supports caller-managed profiles (the generated activate takes
// --driver-version 1, which performs zero profile operations; the self-set
// in writeBoundary is gated on hmDriverVersion < 1), so modern generations
// compose profile-last: panix sets the profile like the standard tiers do,
// and HM's own --rollback is exactly the profile-last revert shape. The
// documented detection contract: gen-version present in the generation's
// build output root with an integer >= 1 means driver-1 support; missing,
// unreadable, or unparseable means the legacy self-setting activate. Legacy
// (driver-0) activate re-setting the same path after panix's restore is a
// no-op (Nix's createGeneration reuses the newest generation for an
// identical store path).

const (
	// hmDriverFlag is the activate argument that selects driver 1: the
	// caller-managed-profile mode where activate performs zero profile
	// operations (HM >= 25.11).
	hmDriverFlag = "--driver-version"
	hmDriverVer  = "1"
)

// hmComposition is the per-deploy home-manager resolution result: the zero
// value keeps today's legacy self-setting composition byte-identically.
type hmComposition struct {
	ProfileLast bool // effective composition tier: standard (profile-last) instead of self-setting
	OldModern   bool // OLD's activate supports --driver-version 1 (revert step arg)
}

// resolveHMComposition resolves the per-deploy composition for one
// home-manager installable from the gen-version probes: NEW modern composes
// profile-last (the standard tier shape) for this deploy; NEW legacy keeps
// today's self-setting composition untouched. The OLD probe only matters in
// the profile-last branch: it decides whether the revert's OLD activation
// step carries the driver flag. The preset row's static tier is unchanged:
// routing sees self-setting either way (both tiers are guarded).
func resolveHMComposition(preset installable.Preset, newModern, oldModern bool) hmComposition {
	if preset.GuardTierValue() != installable.GuardTierSelfSetting || !newModern {
		return hmComposition{}
	}

	return hmComposition{ProfileLast: true, OldModern: oldModern}
}

// composeHMStepLists composes the profile-last home-manager transaction
// (modern HM >= 25.11, the darwin standard shape, spec 4.2): the activation
// child runs --driver-version 1, the commit sets the resolved HM profile to
// the bare activationPackage (the composed NEW is exactly that package, so
// deploy-rs's wrapper/target mismatch is impossible by construction), and
// the revert re-runs OLD's activate with the driver flag when OLD is modern
// (the profile-first restore is the guardian's runtime-derived rule, spec
// 6.5). --set is idempotent, so re-executed lists converge.
func composeHMStepLists(preset installable.Preset, profile, nixEnv, old, new string, oldModern bool) ComposedSteps {
	activation := StepList{{new + "/" + preset.ActivationPath, hmDriverFlag, hmDriverVer}}

	revert := StepList{{old + "/" + preset.ActivationPath}}
	if oldModern {
		revert[0] = append(revert[0], hmDriverFlag, hmDriverVer)
	}

	commit := StepList{{nixEnv, "-p", profile, "--set", new}}

	return ComposedSteps{
		Activation:      activation,
		Commit:          commit,
		Revert:          revert,
		InvariantTarget: new,
		GateEligible:    gateEligibleFor(installable.GuardTierStandard, commit),
	}
}

// parseHMGenVersion applies the detection contract to one gen-version file
// body: integer >= 1 means driver-1 support; missing, unreadable, or
// unparseable content means the legacy self-setting activate.
func parseHMGenVersion(content string) bool {
	version, err := strconv.Atoi(strings.TrimSpace(content))
	if err != nil {
		return false
	}

	return version >= 1
}

// hmProfileResolveScript resolves the HM profile path per HM's own rule and
// prepares the parent profiles directory: the state-home profiles dir wins
// when it exists, the per-user nix store profile is the fallback, and the
// parent profiles dir is created before first use. Runs as the target user
// (su -l), so $HOME/$USER/XDG_STATE_HOME resolve to the deploy identity.
const hmProfileResolveScript = `state="${XDG_STATE_HOME:-$HOME/.local/state}"; dir="$state/nix/profiles"; ` +
	`if [ -d "$dir" ]; then mkdir -p "$dir" && printf %s "$dir/home-manager"; exit 0; fi; ` +
	`pdir="${NIX_STATE_DIR:-/nix/var/nix}/profiles/per-user/${USER:-$(id -un)}"; ` +
	`mkdir -p "$pdir" && printf %s "$pdir/home-manager"`

// probeHMGenerationModern reads one generation closure's gen-version and
// reports driver-1 support. The probe is a quiet read-only exec; the caller
// treats every failure (unreadable, missing, unparseable) as legacy.
func probeHMGenerationModern(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, closure string,
) bool {
	if surface.dryRun() {
		// Dry-run has no live target to probe: previews compose the legacy
		// self-setting shape (documented deviation).
		return false
	}

	var version string

	err := quietExec(surface, "probe home-manager driver support", "failed to probe home-manager driver support",
		targetArgv(mach, preset, targetUser, []string{"cat", closure + "/gen-version"}), &version)
	if err != nil {
		return false
	}

	return parseHMGenVersion(version)
}

// resolveHMProfile resolves the HM profile path per HM's own rule (spec 2's
// documented profile contract): the state-home profiles dir wins when it
// exists, the per-user nix store profile is the fallback, and the parent
// profiles dir is created before first use. Profile-last branch only.
func resolveHMProfile(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser string,
) (string, error) {
	var profile string

	err := quietExec(surface, "resolve the home-manager profile", "failed to resolve the home-manager profile",
		targetArgv(mach, preset, targetUser, []string{"sh", "-c", hmProfileResolveScript}), &profile)
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve the home-manager profile")
	}

	profile = strings.TrimSpace(profile)
	if profile == "" {
		return "", errors.New("failed to resolve the home-manager profile: the probe returned an empty path")
	}

	return profile, nil
}
