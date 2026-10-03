package guard

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/guard"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
)

// This package composes the deployer side of the Activation Guard (spec 4.1, 6.5, 9.3):
// pre-start argv, guardian spawn argv, sweep/decide invocation, and the ctl calls.
// The guardian itself lives in cmd/panix-guard and shares the record/profile protocol
// via internal/guard (single source of truth for both sides).

// Protocol constants shared with the guardian's runtime rules.
const (
	slotBaseSystem = "/run/panix-guard" // system tier slot root (spec 10.1)
	slotBaseDarwin = "/var/root/panix-guard"
	slotBaseUser   = "~/.local/state/panix-guard"
	slotVersionDir = "v2" // slot-layout version component (spec 10.1 coexistence)
	gcRootName     = "gc-root"
	logName        = "log"
)

// SlotDir returns the slot directory for a profile (spec 10.1: one fixed directory per
// profile; name = slug(profile path) + short hash so same-basename profiles cannot
// collide). systemLevel selects the root-owned layout for system-level profiles;
// targetIsDarwin reflects the target platform (darwin slots live in /var/root). The
// path carries the v2 layout component: legacy (pre-v2) slots under the plain root
// become inert and are cleaned up by the pre-start sweep (see sweepLegacySlot).
func SlotDir(profilePath string, systemLevel, targetIsDarwin bool) string {
	base := slotBaseUser

	if systemLevel {
		if targetIsDarwin {
			base = slotBaseDarwin
		} else {
			base = slotBaseSystem
		}
	}

	slug := strings.NewReplacer("/", "-", ".", "-", "~", "").Replace(strings.Trim(profilePath, "/"))
	if slug == "" {
		slug = "profile"
	}
	// User-tier profiles start with ~, which the replacer deletes and its
	// following separator turns into a leading dash; trim it so the slot name
	// stays a plain path segment.
	slug = strings.TrimLeft(slug, "-")

	return fmt.Sprintf("%s/%s/%s-%s", base, slotVersionDir, slug, shortHash(profilePath))
}

// LegacySlotDir returns the legacy (pre-v2) slot path for a v2 slot directory:
// the same root and slot name without the v2 component. A path that is not a
// v2 slot path maps to itself (nothing legacy to derive).
func LegacySlotDir(slotDir string) string {
	parent, name := filepath.Split(filepath.Clean(slotDir))
	// Split keeps the trailing slash on the dir half; clean it so Dir strips
	// the v2 component and not just the empty element.
	parent = filepath.Clean(parent)
	if filepath.Base(parent) != slotVersionDir {
		return slotDir
	}

	return filepath.Join(filepath.Dir(parent), name)
}

// shortHash is a tiny deterministic hash (FNV-1a) rendered as 8 hex chars: stable
// across deploys, collision-safe for profile paths.
func shortHash(s string) string {
	h := uint64(14695981039346656037)
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= 1099511628211
	}

	return fmt.Sprintf("%08x", h&0xffffffff)
}

// ConfirmationGate is the user-facing rollback tier (spec 2).
type ConfirmationGate string

const (
	GateOff   ConfirmationGate = "off"
	GateAuto  ConfirmationGate = "auto"
	GateMagic ConfirmationGate = "magic"
)

// StepList is one panix-composed argv step (spec 6.5).
type StepList = [][]string

// SpawnArgv composes the detached guardian spawn (spec 6.5). newClosure and
// gen come from the deploy composition; the OLD capture happens inside start
// after its convergence (spec 4.1 step 5), so --old is gone from the spawn
// argv and the pre-start mutations ride start's flags instead: --sweep runs
// the classify/converge/truncate sweep under start's flock, --gc-root-target
// pins the transaction's protected closure (absent = no root), --boot-set
// performs the boot-mode pre-start profile set (absent outside boot mode),
// and --nix-store carries the absolute nix-store path the gc-root step needs.
// activation/commit/revert step lists encode the class and mode semantics
// (spec 4.2) and carry absolute paths resolved at pre-start; the guardian
// prefers the transaction's own TXN record lists over these argv lists (spec
// 7), which is what removes the v1.3 recomposition. healthChecksLocal is the
// guardian-side check list (spec 2: auto and magic tiers gate the commit on
// it). tier comes from the preset's GuardTier field: the preset table is the
// single source of truth for per-type semantics.
func SpawnArgv(
	guardBin, dir, key, profile, nixEnv, nixStore, newClosure string,
	gen int64, mode string, tier installable.GuardTier, gate ConfirmationGate,
	activationTimeout, confirmTimeout time.Duration, rebootOnRevertFailure bool,
	healthChecksLocal []string,
	activation, commit, revert StepList, invariantTarget string,
	gcRootTarget, bootSet string, sweep bool,
) []string {
	argv := []string{
		guardBin, "start",
		"--dir", dir,
		"--key", key,
		"--profile", profile,
		"--nix-env", nixEnv,
		"--new", newClosure,
		"--gen", strconv.FormatInt(gen, 10),
		"--mode", mode,
		"--tier", string(tier),
		"--confirmation", string(gate),
		"--activation-timeout", activationTimeout.String(),
		"--confirm-timeout", confirmTimeout.String(),
		"--reboot-on-revert-failure=" + strconv.FormatBool(rebootOnRevertFailure),
		"--health-checks-local", mustJSON(healthChecksLocal),
		"--builtin-unit-check=" + strconv.FormatBool(tier == installable.GuardTierFull),
		"--activation-argv", mustJSON(activation),
		"--commit-argv", mustJSON(commit),
		"--revert-argv", mustJSON(revert),
		"--invariant-target", invariantTarget,
		// The bool composes in =form: a bare --sweep followed by the next
		// flag would be misparsed by Go's flag package (the v1.3 gotcha).
		"--sweep=" + strconv.FormatBool(sweep),
	}

	if gcRootTarget != "" {
		argv = append(argv, "--gc-root-target", gcRootTarget)
	}

	if bootSet != "" {
		argv = append(argv, "--boot-set", bootSet)
	}

	return append(argv, "--nix-store", nixStore)
}

// CtlArgv composes a control-channel call (spec 8). Runs over a fresh connection for
// the magic tier: the reachability proof and the confirm delivery are the same exec.
// cause rides the request's --cause narration (spec 8's give-up path): empty composes
// the existing flag-less shape byte-identically, and the cause never changes the
// per-verb exit-code contract (0/5/3/4/7).
func CtlArgv(guardBin, dir, key, command string, wait time.Duration, cause string) []string {
	argv := []string{guardBin, "ctl", "--dir", dir, "--key", key, "--wait", wait.String()}

	if cause != "" {
		argv = append(argv, "--cause", cause)
	}

	return append(argv, command)
}

// GuardCancelCause is the cause narration the deployer's give-up path sends
// with its one best-effort fresh-connection revert-request (spec 8).
const GuardCancelCause = "cancel"

// InspectArgv composes the one-shot slot verdict (spec 9.5): one JSON object
// with the derived state and the lock flag, which stays valid even for legacy
// or unparseable logs. It replaces the v1.3 follow/attach viewers and the
// lock probe verb.
func InspectArgv(guardBin, dir string) []string {
	return []string{guardBin, "inspect", "--dir", dir}
}

// inspectAbsentMarker is the stdout marker InspectProbeArgv's script prints
// when the slot or its log is missing. Inspect's exit 4 is the expected-clean
// outcome on fresh targets (no slot, no log, so no lock can exist), so the
// wrapper folds it into a zero exit instead of logging ERR into the phase
// logs - the same treatment the existence probes get. Every other exit code
// passes through unchanged, so genuine failures stay loud and fail closed.
const inspectAbsentMarker = "absent"

// InspectProbeArgv wraps the inspect one-shot (InspectArgv) in an sh -c
// script that demotes the expected-clean absent outcome (exit 4) to exit 0
// with inspectAbsentMarker on stdout; the caller classifies the marker.
func InspectProbeArgv(guardBin, dir string) []string {
	script := shellJoin(InspectArgv(guardBin, dir)) + "\n" +
		"rc=$?\n" +
		"if [ \"$rc\" -eq " + strconv.Itoa(guard.InspectExitNoSlot) + " ]; then printf " + inspectAbsentMarker + "; exit 0; fi\n" +
		"exit \"$rc\""

	return []string{"sh", "-c", script}
}

// shellJoin renders an argv as one POSIX shell command line (every word
// single-quoted), so guard one-shots can embed in sh -c probe scripts.
func shellJoin(argv []string) string {
	words := make([]string, len(argv))
	for i, word := range argv {
		words[i] = shellquote.QuoteWord(word)
	}

	return strings.Join(words, " ")
}

// ConvergeArgv composes the offline convergence call (spec 9.3): converge
// self-locks, classifies from the log tail, and converges a non-terminal
// transaction from the transaction's own TXN record lists (argv fallback and
// generation arithmetic behind it), so no step lists ride the argv. The
// profile path and the resolved nix-env ride along as the context fallback
// (the post-mortem must restore the profile even when the records carry no
// pf; the fold's value wins when present, the argv fills the rest). Empty
// values are omitted: the converge flags are optional hints. Truncate clears
// the log after a converged terminal state (the pre-start caller; inline
// post-mortem keeps the log for reporting).
func ConvergeArgv(guardBin, dir, profile, nixEnv string, truncate bool) []string {
	argv := []string{guardBin, "converge", "--dir", dir}

	if profile != "" {
		argv = append(argv, "--profile", profile)
	}

	if nixEnv != "" {
		argv = append(argv, "--nix-env", nixEnv)
	}

	if truncate {
		argv = append(argv, "--truncate")
	}

	return argv
}

// mustJSON serializes spawn argv payload fields; these values are built in-process and
// always serialize. Nil slices normalize to "[]": the guardian's parsers accept null,
// but the argv contract stays explicit.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return "[]"
	}

	return string(b)
}

// InspectVerdict is panix-guard inspect's one JSON verdict (spec 9.5): the
// derived slot state plus the lock flag. The lock state is valid even when
// the log is legacy (@PG1) or unparseable beyond noise (status legacy or
// unknown); the verdict's full shape also carries old/new/gen/mode/tier/rc/
// err/key and the embedded TXN lists, which panix consumes through the
// records instead of this verdict.
type InspectVerdict struct {
	Status   string `json:"status"`
	Terminal bool   `json:"terminal"`
	Err      string `json:"err,omitempty"`
	Lock     bool   `json:"lock"`
}
