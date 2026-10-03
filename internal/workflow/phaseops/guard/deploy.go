package guard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/guard"
	logs_command "github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/pkg/errors"
)

// This file wires the deployer side of the Activation Guard into the activate
// phase (spec 4.1, 8, 9.1-9.4, 10.2): the pre-start sequence (resolution,
// probes, legacy sweep, transfer, lock probe), the guardian spawn whose exec
// is the duplex live relay (start performs every pre-start mutation under its
// own flock: sweep, gc root, boot set, OLD capture), the magic-tier confirm
// loop over the wire with the fresh-connection floors, and the exit-code
// outcome mapping (spec 9.1).

// Protocol constants shared with cmd/panix-guard (keep in sync).
const (
	guardianBinaryName = "panix-guard" // target-side binary name inside the slot
	guardianTmpName    = ".guardian.tmp"

	// The exit-code vocabulary lives in internal/guard (exitcodes.go, spec
	// 9.5): the per-verb blocks are the single source of truth for both
	// sides, and every panix-side switch consumes those constants.

	// Guardian state names as written into STATE and terminal records (spec
	// 6.2; keep in sync with cmd/panix-guard's txState values).
	statusActivated          = "activated"
	statusCommitted          = "committed"
	statusReverted           = "reverted"
	statusRevertFailed       = "revert_failed"
	statusFailedPrecondition = "failed_precondition"
	statusActivationExited   = "activation_exited"

	// dryRunOldClosure is the dry-run placeholder for the rollback-target
	// hint, so previews compose the same argv shapes a real deploy does.
	dryRunOldClosure = "/nix/store/dry-run-old-closure"
)

// panix-side timing bounds (spec 2, 6.2, 9.4; keep in sync with
// cmd/panix-guard's constants where noted).
const (
	remoteCheckTimeout  = 30 * time.Second // per-check bound (spec 2 default)
	confirmGiveUpSlack  = 60 * time.Second // confirm-loop give-up margin past the window
	commitBudgetFloor   = 120 * time.Second
	deadlineGrace       = 30 * time.Second
	spawnExecSlackFloor = 5 * time.Minute
	spawnExecMargin     = 30 * time.Second
	ctlWait             = 5 * time.Second // guardian's ctl --wait default (cmd/panix-guard ctl.go)
	ctlExecTimeout      = 3 * ctlWait     // exec bound above the ctl's own wait

	// inspectPollTimeout bounds one inspect one-shot in the post-disconnect
	// poll loop (spec 9.4): the machine just reconnected, so a quick verdict
	// is expected and a stalled exec must not hold the loop.
	inspectPollTimeout = 15 * time.Second

	// Wire-window cadence (spec 8, 3): the wire is the primary transport and
	// the fresh-connection log tail stays as the floor. The loop wakes on
	// wire records; the floor fires only after floorIdle of wire silence.
	floorCheckInterval = time.Second
	floorIdle          = 10 * time.Second
)

// Target platform labels (uname -s values that matter for slot placement).
const (
	platformLinux  = "Linux"
	platformDarwin = "Darwin"
)

// guardianBinaryPath is the transferred binary's target-side path inside the slot.
func guardianBinaryPath(slotDir string) string {
	return slotDir + "/" + guardianBinaryName
}

// guardBinary returns the control-host path of the gzip-compressed guardian
// binary for the target architecture (spec 10.3, 15). The flake populates the
// embed directory at build time (T7); until then every guarded deploy fails
// loudly here instead of transferring a missing binary.
func guardBinary(targetArch string) (string, error) {
	return "", errors.Errorf("guardian embed not built (T7): no embedded guardian for arch %q", targetArch)
}

// targetArgv applies the installable's privilege model to a target-side argv
// (spec 16): system-level presets elevate with MaybeSudo, a target user runs
// through su -l, the SSH user stays unwrapped.
func targetArgv(mach *machine.Machine, preset installable.Preset, targetUser string, argv []string) []string {
	return phaseops.WrapAsTargetUser(mach, preset, targetUser, argv)
}

// execSurface is the exec surface the pre-start sequence and the log-tail
// readers need: one exec primitive with output capture, the pipe primitive
// for the binary transfer, and the dry-run flag. The production adapter
// wraps the phase executioner; the pre-start order tests substitute a
// scripted surface so the step sequence is observable without a transport.
type execSurface interface {
	run(description, statusIfRunning, statusIfFailed string, argv []string, opts ...executioner.ExecOption) (string, error)
	runPipe(description, statusIfRunning, statusIfFailed string, spec executioner.PipeSpec, opts ...executioner.ExecOption) error
	dryRun() bool
}

// lockedBuffer collects output-tap chunks delivered from the executioner's
// read goroutine; the mutex keeps concurrent chunk writes safe and the read
// happens after Exec returns.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) write(chunk []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buf.Write(chunk)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// executionerSurface adapts the phase executioner to execSurface. run adds
// the output-capture hooks ahead of the caller's options, so a caller-provided
// OnDryRun still overrides the no-op default and any caller OnSuccess keeps
// overriding the capture, exactly like the pre-adapter quietExec contract.
type executionerSurface struct {
	ex *executioner.Executioner
}

func (s executionerSurface) run(
	description, statusIfRunning, statusIfFailed string,
	argv []string,
	opts ...executioner.ExecOption,
) (string, error) {
	var (
		capture lockedBuffer
		out     string
	)

	err := s.ex.Exec(
		description,
		statusIfRunning,
		statusIfFailed,
		argv,
		slices.Concat([]executioner.ExecOption{
			// Output capture rides the output tap as well as OnSuccess: the
			// tap fires for every chunk regardless of the exit status, so a
			// FAILING command's captured output (the inspect verdict on
			// exit 1, spec 9.5) still reaches the caller, which the
			// OnSuccess-only capture cannot do. Caveat: the tap is a
			// single-slot option, so a caller-provided WithOutputTap
			// overrides this capture (no current caller does).
			executioner.WithOutputTap(capture.write),
			executioner.OnSuccess(func(log *logs_command.CommandLog) error {
				out = strings.TrimSpace(log.Output.String())

				return nil
			}),
			executioner.OnDryRun(func() {}),
		}, opts)...,
	)
	if out == "" {
		out = strings.TrimSpace(capture.String())
	}

	return out, err //nolint:wrapcheck // pass-through surface; callers annotate
}

func (s executionerSurface) runPipe(
	description, statusIfRunning, statusIfFailed string,
	spec executioner.PipeSpec,
	opts ...executioner.ExecOption,
) error {
	return s.ex.ExecPipe(description, statusIfRunning, statusIfFailed, spec, opts...) //nolint:wrapcheck // pass-through; callers annotate
}

func (s executionerSurface) dryRun() bool {
	return s.ex.DryRun()
}

// quietExec runs one target-side command without registering it in the phase
// log: the pre-start sequence must not flood the TUI, while output capture and
// exit handling stay fully functional. stdout lands (trimmed) in *out; a nil
// out discards it. Dry-run skips the assignment: the caller's OnDryRun hooks
// may have filled the pointer already, and the empty capture must not clobber
// them. Extra options apply after the base hooks, so a caller-provided
// OnDryRun overrides the no-op default.
func quietExec(
	surface execSurface,
	description, statusIfFailed string,
	argv []string,
	out *string,
	opts ...executioner.ExecOption,
) error {
	hooks := append([]executioner.ExecOption{
		executioner.Quiet(),
		executioner.OnDryRun(func() {}),
	}, opts...)

	captured, err := surface.run(description, description, statusIfFailed, argv, hooks...)
	if out != nil && !surface.dryRun() {
		*out = captured
	}

	return err
}

// exitCodeOfErr extracts the process exit code from an exec error chain.
func exitCodeOfErr(err error) (int, bool) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}

	return 0, false
}

// targetPathExists probes one target path with test(1): true when present,
// false for the plain negative probe (exit 1), and an error for anything else
// (fail closed: an unprobeable target is not a fresh slot). Dry-run previews.
func targetPathExists(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, description, path string,
) (bool, error) {
	if surface.dryRun() {
		return false, nil
	}

	// The probe never fails the exec on the expected-clean outcome: absence
	// is reported on stdout and the shell always exits 0, so a fresh slot
	// (the expected case) does not scream ERR into the phase logs. A genuine
	// exec failure still fails closed.
	script := "if [ -e " + shellquote.QuoteWord(path) + " ]; then printf yes; else printf no; fi"

	out, err := surface.run(
		description,
		description,
		description+" failed",
		targetArgv(mach, preset, targetUser, []string{"sh", "-c", script}),
		executioner.Quiet(),
		executioner.OnDryRun(func() {}),
	)
	if err != nil {
		return false, err //nolint:wrapcheck // the caller annotates with the failing step
	}

	return strings.TrimSpace(out) == "yes", nil
}

// resolveCommandPathQuiet resolves a command on the target PATH un-elevated
// (the activate handler's resolveCommandPath precedent, quiet): sudo's
// secure_path excludes the nix profile directories, absolute paths pass
// through unchanged.
func resolveCommandPathQuiet(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, cmdName string,
) (string, error) {
	if strings.HasPrefix(cmdName, "/") {
		return cmdName, nil
	}

	var resolved string

	err := quietExec(
		surface,
		"resolve "+cmdName,
		"failed to resolve "+cmdName,
		targetArgv(mach, preset, targetUser, []string{"sh", "-c", "command -v -- " + shellquote.Quote(cmdName)}),
		&resolved,
		executioner.OnDryRun(func() {
			resolved = cmdName
		}),
	)
	if err != nil {
		return "", errors.Wrapf(err, "%s resolution failed", cmdName)
	}

	if resolved == "" {
		return "", errors.Errorf("%s not found on PATH on the target", cmdName)
	}

	return resolved, nil
}

// newDeployKey builds the per-deploy record nonce (spec 7): unix nanos plus 6
// random bytes, hex-encoded.
func newDeployKey() (string, error) {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.Wrap(err, "deploy key entropy failed")
	}

	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(raw[:])), nil
}

// targetPlatform detects the target OS for slot placement (spec 10.1: darwin
// slots live in /var/root, everything else on linux roots). Machine carries no
// platform field, so the probe is a quiet uname -s cached per machine; a
// failed probe falls back to the output type so dry-run previews stay honest.
func targetPlatform(surface execSurface, mach *machine.Machine, outputType string) string {
	if !surface.dryRun() {
		if cached, ok := platformCache.Load(mach); ok {
			return cached.(string) //nolint:forcetypeassert // the cache only stores strings
		}
	}

	out, err := surface.run("detect platform", "detect platform", "platform detection failed",
		[]string{"uname", "-s"}, executioner.Quiet(), executioner.OnDryRun(func() {}))
	if err == nil {
		switch {
		case strings.Contains(out, platformDarwin):
			if !surface.dryRun() {
				platformCache.Store(mach, platformDarwin)
			}

			return platformDarwin
		case strings.Contains(out, platformLinux):
			if !surface.dryRun() {
				platformCache.Store(mach, platformLinux)
			}

			return platformLinux
		}
	}

	if outputType == "darwinConfigurations" {
		return platformDarwin
	}

	return platformLinux
}

// platformCache memoizes the uname -s probe per machine so sequential guarded
// deploys against one machine do not re-probe. Dry-run results are never
// stored: a real deploy in the same process must probe for real.
var platformCache sync.Map // *machine.Machine -> string

// targetArch resolves the target architecture for the guardian embed (spec
// 10.3): Inspect probed uname -m; without it the probe reruns quietly.
func targetArch(surface execSurface, mach *machine.Machine) (string, error) {
	if mi := mach.MetaInspect.Load(); mi != nil && mi.Architecture.String() != "" {
		return mi.Architecture.String(), nil
	}

	out, err := surface.run("detect architecture", "detect architecture", "architecture detection failed",
		[]string{"uname", "-m"}, executioner.Quiet(),
		executioner.OnDryRun(func() {}))
	if err != nil {
		return "", errors.Wrap(err, "target architecture unavailable for the guardian embed")
	}

	out = strings.TrimSpace(out)
	if out == "" {
		if surface.dryRun() {
			return "DRY_RUN", nil
		}

		return "", errors.New("target architecture unavailable for the guardian embed: uname -m returned nothing")
	}

	return out, nil
}

// spawnExecTimeout bounds the streaming spawn exec: the guardian's internal
// deadline (spec 6.2) is activation + confirm + commit budget + grace, and the
// viewer must outlive it so the outcome is always observed panix-side. The
// floor stays the prescribed 5m slack; a long activation stretches the budget
// (max(120s, activation/2)) and the slack grows with it.
func spawnExecTimeout(activationTimeout, confirmTimeout time.Duration) time.Duration {
	budget := max(activationTimeout/2, commitBudgetFloor)

	slack := spawnExecSlackFloor
	if grace := budget + deadlineGrace; grace > slack {
		slack = grace
	}

	return activationTimeout + confirmTimeout + slack + spawnExecMargin
}

// probeKillUserProcesses enforces the KUP contract (spec 10.2): logind kills
// the whole session scope on disconnect (a Setsid'd guardian included), so a
// target with KillUserProcesses=yes hard-fails before any mutation. The probe
// runs on every Linux target (NixOS defaults to no but users can override);
// darwin has no logind and skips. An unavailable probe fails closed on Linux.
func probeKillUserProcesses(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, platform string,
) error {
	if platform == platformDarwin {
		return nil
	}

	busctl, err := resolveCommandPathQuiet(surface, mach, preset, targetUser, "busctl")
	if err != nil {
		return errors.Wrap(err, "KillUserProcesses probe unavailable (fail closed): busctl not found on the target")
	}

	var out string

	err = quietExec(
		surface,
		"KillUserProcesses probe",
		"KillUserProcesses probe failed",
		targetArgv(mach, preset, targetUser, []string{
			busctl, "get-property", "org.freedesktop.login1", "/org/freedesktop/login1",
			"org.freedesktop.login1.Manager", "KillUserProcesses",
		}),
		&out,
	)
	if err != nil {
		return errors.Wrap(err, "KillUserProcesses probe unavailable (fail closed)")
	}

	if strings.Contains(out, "true") {
		return errors.New("target has logind KillUserProcesses=yes: the session-scope kill would take down the detached guardian on disconnect; set KillUserProcesses=no in logind.conf (lingering does not help) or deploy with rollback: off") //nolint:lll
	}

	return nil
}

// inspectSlot runs one panix-guard inspect one-shot and parses the verdict
// (spec 9.5): one JSON object with the derived state and the lock flag. The
// probe wrapper (InspectProbeArgv) folds the expected-clean absent outcome
// (exit 4: no slot or log) into a zero exit with the absent marker on stdout,
// so fresh targets stop logging ERR into the phase logs; the marker reads as
// the absent verdict (found=false, lock=false), so callers distinguish
// absence via found. Exit 1 means the verdict was still printed (legacy or
// unparseable log) and its lock state is valid, so it parses and returns;
// only a failure with no parseable verdict fails closed.
func inspectSlot(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, description, guardBin, dir string,
	opts ...executioner.ExecOption,
) (InspectVerdict, bool, error) {
	if surface.dryRun() {
		return InspectVerdict{}, false, nil // dry-run previews assume a clean slot
	}

	var out string

	err := quietExec(surface, description, description+" failed",
		targetArgv(mach, preset, targetUser, InspectProbeArgv(guardBin, dir)), &out, opts...)
	if err != nil {
		// Exit 1 means the guardian still printed a verdict (legacy or
		// unparseable log); its lock state is valid (spec 9.5), so parse
		// and return it instead of failing closed. The absent outcome (4)
		// never surfaces as an error: the probe wrapper demotes it.
		if code, ok := exitCodeOfErr(err); ok && code == guard.InspectExitUnparseable {
			var v InspectVerdict

			jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &v)
			if jsonErr == nil {
				return v, true, nil
			}
		}

		return InspectVerdict{}, false, err //nolint:wrapcheck // callers annotate with the failing step
	}

	if strings.TrimSpace(out) == inspectAbsentMarker {
		return InspectVerdict{}, false, nil // the absent outcome, quiet (no slot or log)
	}

	var verdict InspectVerdict

	err = json.Unmarshal([]byte(strings.TrimSpace(out)), &verdict)
	if err != nil {
		return InspectVerdict{}, false, errors.Wrap(err, "guard slot inspect verdict unparseable")
	}

	return verdict, true, nil
}

// probeSlotLock fails fast when a live transaction owns the slot (spec 4.1
// step 4): one inspect one-shot carries the lock state, derived from the
// log's flock and valid even for legacy or unparseable logs. A missing slot
// or log (inspect exit 4) means no lock can exist. Any other inspect failure
// refuses to spawn (fail closed).
func probeSlotLock(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir string,
) error {
	verdict, found, err := inspectSlot(surface, mach, preset, targetUser,
		"guard slot lock probe", guardianBinaryPath(slotDir), slotDir)
	if err != nil {
		return errors.Wrap(err, "fail closed")
	}

	if !found {
		return nil // no slot or log: no lock can exist (spec 4.1 step 4)
	}

	if verdict.Lock {
		return errors.New(liveTransactionMessage(slotDir, verdict))
	}

	return nil
}

// legacyPlan is the pre-start action for a legacy (pre-v2) guard slot
// (spec 10.1 coexistence): the v2 layout moves slots under the version
// component, so legacy slots become inert and are either cleared or refused.
type legacyPlan int

const (
	legacySkip       legacyPlan = iota // absent: nothing to do
	legacyRemove                       // free: remove the legacy slot tree
	legacyLive                         // locked: fail fast on the live transaction
	legacyFailClosed                   // unprobeable: refuse to touch or proceed
)

// legacyFacts is what the pre-start sequence observes about a legacy slot.
type legacyFacts struct {
	dirExists  bool
	logMissing bool  // inspect exit 4: no log inode, so no lock can exist
	locked     bool  // the verdict's lock flag (valid whenever the verdict printed)
	inspectErr error // the inspect failure (exit 1 or transport): fail closed
}

// legacySweepPlan maps the observed legacy-slot facts to the pre-start action.
// The lock state comes from the transferred v2 guardian's inspect running
// against the legacy directory: the flock lives on the log's open file
// description (format-independent), and inspect reports status legacy or
// unknown with a valid lock for @PG1 logs. A missing log inode means no lock
// can exist; anything else unprobeable fails closed.
func legacySweepPlan(f legacyFacts) legacyPlan {
	switch {
	case !f.dirExists:
		return legacySkip
	case f.inspectErr != nil:
		return legacyFailClosed
	case f.logMissing:
		return legacyRemove
	case f.locked:
		return legacyLive
	default:
		return legacyRemove
	}
}

// sweepLegacySlot handles the legacy (pre-v2) slot of the same root in the
// pre-start sequence: a live legacy transaction fails the deploy exactly like
// a live v2 slot, a free legacy slot is removed with its whole tree (log,
// gc-root symlink, persisted guardian binary), and an unprobeable slot refuses
// to proceed (fail closed). The lock state comes from the transferred v2
// binary's inspect against the legacy directory (the legacy @PG1 binary has no
// inspect verb), which is why the sweep runs after the guardian transfer; the
// transfer is idempotent and hash-skipped, so the fail-fast cost is a no-op
// transfer at worst. Dry-run previews assume a clean target, exactly like the
// v2 lock probe.
func sweepLegacySlot(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir, legacyDir string,
) error {
	if surface.dryRun() {
		return nil
	}

	dirExists, err := targetPathExists(surface, mach, preset, targetUser, "legacy guard slot probe", legacyDir)
	if err != nil {
		return errors.Wrap(err, "fail closed")
	}

	verdict, found, inspectErr := inspectSlot(surface, mach, preset, targetUser,
		"legacy guard slot lock probe", guardianBinaryPath(slotDir), legacyDir)

	switch legacySweepPlan(legacyFacts{
		dirExists:  dirExists,
		logMissing: inspectErr == nil && !found,
		locked:     verdict.Lock,
		inspectErr: inspectErr,
	}) {
	case legacySkip:
		return nil
	case legacyRemove:
		return removeLegacySlot(surface, mach, preset, targetUser, legacyDir)
	case legacyLive:
		return errors.New(legacyLiveMessage(legacyDir))
	default:
		if inspectErr != nil {
			return errors.Wrap(inspectErr, "fail closed")
		}

		return errors.Errorf("legacy guard slot %s is unprobeable (fail closed)", legacyDir)
	}
}

// legacyLiveMessage renders the fail-fast pointer for a locked legacy slot: a
// live pre-v2 transaction must be finished or cleared with the old panix.
func legacyLiveMessage(legacyDir string) string {
	return "live deploy, slot " + legacyDir + ": a legacy (pre-v2) panix guard transaction is live; finish or clear it with the old panix version"
}

// removeLegacySlot deletes the legacy slot directory tree. Stale
// /nix/var/nix/gcroots/auto entries that point into the removed tree self-clean
// once their target vanishes (verified Nix behavior: the gcroots sweeper drops
// dead auto links).
func removeLegacySlot(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, legacyDir string,
) error {
	return quietExec(surface, "legacy guard slot cleanup", "legacy guard slot cleanup failed",
		targetArgv(mach, preset, targetUser, []string{"rm", "-rf", "--", legacyDir}), nil)
}

// liveTransactionMessage renders the fail-fast pointer for a locked slot
// (spec 4.1 step 4): the inspect verdict's last status, or the honest unknown
// form. The verdict carries no guardian pid, so the message stays on the
// contract's status form.
func liveTransactionMessage(slotDir string, verdict InspectVerdict) string {
	return fmt.Sprintf("live deploy, slot %s: last status %s", slotDir, statusOrUnknown(verdict.Status))
}

// statusOrUnknown renders a transaction status for reports.
func statusOrUnknown(status string) string {
	if status == "" {
		return "unknown"
	}

	return status
}

// readLogTail reads the slot log tail (spec 9.3: the last 64KiB carry current
// state plus OLD/NEW/gen). Post-start readers use fresh connections: the
// multiplexed master may have died with the viewer.
func readLogTail(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir string,
	fresh bool,
) (string, error) {
	opts := []executioner.ExecOption{}
	if fresh {
		opts = append(opts, executioner.FreshConnection())
	}

	var tail string

	err := quietExec(
		surface,
		"guard log tail",
		"guard log tail read failed",
		targetArgv(mach, preset, targetUser, []string{
			"tail", "-c", strconv.FormatInt(guard.LogTailScanBytes, 10), slotDir + "/" + logName,
		}),
		&tail,
		opts...,
	)

	return tail, err //nolint:wrapcheck // callers annotate with the failing step
}

// rollbackHint resolves the argv-composition rollback target and generation
// (spec 6.5: the revert/invariant argv shapes stay panix-composed): the
// closure of the machine's current generation from the inspect-era inventory.
// It is a hint, not the capture the v1.3 deployer performed: the authoritative
// OLD is the guardian's own post-converge capture (spec 4.1 step 5), which
// start performs under its flock and streams back in the HELLO/TXN records;
// panix derives its reporting context from those records. The hint is exact
// whenever the profile still points at the inspected generation and is
// corrected guardian-side otherwise (the transaction's own TXN lists and the
// runtime profile restore, spec 6.5).
func rollbackHint(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, profile string,
) (string, int64, error) {
	var current uint

	if mi := mach.MetaInspect.Load(); mi != nil && mi.Generations != nil {
		current = mi.Generations.Current
	}

	if surface.dryRun() {
		return dryRunOldClosure, int64(current), nil
	}

	if current == 0 {
		return "", 0, errors.New("rollback-target hint unavailable: the machine's generation inventory is empty (fail closed)")
	}

	var oldClosure string

	err := quietExec(surface, "resolve rollback target", "failed to resolve the rollback target",
		targetArgv(mach, preset, targetUser, []string{
			"readlink", fmt.Sprintf("%s-%d-link", profile, current),
		}), &oldClosure)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to resolve the rollback-target hint")
	}

	if oldClosure == "" {
		return "", 0, errors.New("rollback-target hint is empty: no generation closure to compose the transaction against")
	}

	return oldClosure, int64(current), nil
}

// transferGuardian streams the embedded guardian into the slot atomically
// (spec 4.1 step 3): the payload lands in a temp file that renames into place,
// so a truncated transfer is never executed; the probe keeps unchanged targets
// on the same inode. Elevation rides on the probe/write argv per the
// transferCommandPipeSpec pattern (spec 16).
func transferGuardian(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, embedPath, slotDir string,
) error {
	binPath := guardianBinaryPath(slotDir)

	return surface.runPipe(
		"transfer guardian",
		"transferring guardian binary",
		"guardian transfer failed",
		executioner.PipeSpec{
			Source: []string{"cat", embedPath},
			Probe:  targetArgv(mach, preset, targetUser, []string{"sh", "-c", guardianProbeScript(binPath)}),
			Write:  targetArgv(mach, preset, targetUser, []string{"sh", "-c", guardianWriteScript(slotDir)}),
		},
	)
}

// guardianProbeScript reports the transferred binary's sha256 when it exists;
// any probe failure means write, never transfer failure.
func guardianProbeScript(binPath string) string {
	quoted := shellquote.QuoteWord(binPath)

	return "set -e\nif [ -e " + quoted + " ]; then\n  sha256sum " + quoted + "\nfi\n"
}

// guardianWriteScript streams stdin into the slot and renames atomically;
// umask 077 keeps the bytes private until the explicit owner-only chmod.
func guardianWriteScript(slotDir string) string {
	tmp := shellquote.QuoteWord(slotDir + "/" + guardianTmpName)
	bin := shellquote.QuoteWord(guardianBinaryPath(slotDir))

	return "set -e\numask 077\ncat > " + tmp + "\nchmod 0700 " + tmp + "\nmv -f " + tmp + " " + bin + "\n"
}

// guardedPlan carries everything the pre-start sequence resolved for the
// spawn window (spec 4.1 step 5): the transaction composition, the mutation
// targets that ride start's argv, and the identities the window needs. The
// caller that runs the window supplies the executioner (the phase executioner
// in production, a local executioner in the wire tests).
type guardedPlan struct {
	mach              *machine.Machine
	preset            installable.Preset
	targetUser        string
	slotDir           string
	guardBin          string
	key               string
	gate              ConfirmationGate
	activationTimeout time.Duration
	confirmTimeout    time.Duration
	spawnArgv         []string
	// profile and nixEnv are the deploy-resolved identities the post-mortem
	// converge argv needs as its context fallback (the fold restores the
	// profile when the records carry it; nix-env exists only here).
	profile string
	nixEnv  string
}

// guardTools bundles the resolved identities and paths the pre-start sequence
// and the spawn window need (spec 4.1 step 1).
type guardTools struct {
	embedPath string
	nixEnv    string
	nixStore  string
	key       string
}

// resolveGuardTools resolves the target architecture, the guardian embed, the
// absolute nix tool paths and the per-deploy key (spec 4.1 step 1).
func resolveGuardTools(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, platform string,
	embedResolver func(targetOS, targetArch string) (string, error),
) (guardTools, error) {
	arch, err := targetArch(surface, mach)
	if err != nil {
		return guardTools{}, err
	}

	embedPath, err := resolveEmbedPath(surface, platform, arch, embedResolver)
	if err != nil {
		return guardTools{}, err
	}

	nixEnv, err := resolveCommandPathQuiet(surface, mach, preset, targetUser, "nix-env")
	if err != nil {
		return guardTools{}, err
	}

	nixStore, err := resolveCommandPathQuiet(surface, mach, preset, targetUser, "nix-store")
	if err != nil {
		return guardTools{}, err
	}

	key, err := newDeployKey()
	if err != nil {
		return guardTools{}, err
	}

	return guardTools{embedPath: embedPath, nixEnv: nixEnv, nixStore: nixStore, key: key}, nil
}

// prepareGuardSlot creates the slot directory (spec 4.1 step 1); the
// guardian's start re-verifies ownership under the lock (spec 4.1 step 2, 10.1).
func prepareGuardSlot(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir string,
) error {
	return quietExec(surface, "prepare guard slot", "guard slot preparation failed",
		targetArgv(mach, preset, targetUser, []string{"mkdir", "-m", "0700", "-p", slotDir}), nil)
}

// prepareGuardedActivation runs the pre-start sequence (spec 4.1 steps 1-4):
// resolution and slot preparation, the read-only probes, the legacy sweep, the
// guardian transfer (before the lock probe, so the probe always has a current
// binary) and the lock probe, then composes the spawn window. Every pre-start
// mutation of the v1.3 sequence (sweep, gc root, OLD capture) moved under
// start's flock (spec 4.1 step 5) and rides the spawn argv instead. The
// surface parameter is the exec surface (the phase executioner in production)
// and embedResolver resolves the guardian embed (GuardBinary in production);
// tests substitute both to observe the sequence without a transport.
func prepareGuardedActivation(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	outputType, targetUser, newClosure, mode string,
	embedResolver func(targetOS, targetArch string) (string, error),
) (guardedPlan, error) {
	tier := preset.GuardTierValue()
	gate := ConfirmationGate(mach.Rollback.Get())
	activationTimeout := mach.GetActivationTimeout()
	confirmTimeout := mach.GetRollbackConfirmTimeout()

	// (1) Resolve and prepare (read-only plus slot creation, spec 4.1 step 1).
	platform := targetPlatform(surface, mach, outputType)
	slotDir := SlotDir(preset.ProfilePath, preset.IsSystemLevelValue(), platform == platformDarwin)

	tools, err := resolveGuardTools(surface, mach, preset, targetUser, platform, embedResolver)
	if err != nil {
		return guardedPlan{}, err
	}

	err = prepareGuardSlot(surface, mach, preset, targetUser, slotDir)
	if err != nil {
		return guardedPlan{}, err
	}

	// (2) KUP probe (read-only, spec 4.1 step 2).
	err = probeKillUserProcesses(surface, mach, preset, targetUser, platform)
	if err != nil {
		return guardedPlan{}, err
	}

	// (3) Transfer the guardian (spec 4.1 step 3), before both lock probes so
	// they always have a current binary to run.
	err = transferGuardian(surface, mach, preset, targetUser, tools.embedPath, slotDir)
	if err != nil {
		return guardedPlan{}, err //nolint:wrapcheck // statusIfFailed annotates the transfer
	}

	// (3a) Legacy sweep (spec 4.1 step 2 coexistence, after the transfer): the
	// inspect-based legacy lock probe runs the transferred v2 binary against
	// the legacy directory, because the legacy @PG1 binary has no inspect verb.
	// The transfer is idempotent and hash-skipped, so the fail-fast cost on a
	// live legacy transaction is a no-op transfer at worst.
	err = sweepLegacySlot(surface, mach, preset, targetUser, slotDir, LegacySlotDir(slotDir))
	if err != nil {
		return guardedPlan{}, err
	}

	// (4) Lock probe (spec 4.1 step 4, HARD gate, unchanged semantics).
	err = probeSlotLock(surface, mach, preset, targetUser, slotDir)
	if err != nil {
		return guardedPlan{}, err
	}

	// (4a) Rollback-target hint for the argv composition (see rollbackHint).
	// The profile starts from the preset row (the slot above keys off it,
	// one slot per logical HM profile across both compositions).
	profile := preset.ProfilePath

	// Home-manager resolves its per-deploy composition first (spec 4.2 tier
	// table): modern HM (>= 25.11, NEW's gen-version >= 1) composes
	// profile-last, so the hint resolves against the HM-rule profile
	// (resolved and its parent profiles dir created in one quiet exec) and
	// the OLD generation is probed for the revert's driver flag. Legacy HM
	// keeps today's self-setting composition byte-identically; dry-run has
	// no live target to probe and previews the legacy shape.
	var hmRes hmComposition

	if preset.GuardTierValue() == installable.GuardTierSelfSetting && !surface.dryRun() {
		newModern := probeHMGenerationModern(surface, mach, preset, targetUser, newClosure)

		if newModern {
			resolvedProfile, resolveErr := resolveHMProfile(surface, mach, preset, targetUser)
			if resolveErr != nil {
				return guardedPlan{}, resolveErr
			}

			profile = resolvedProfile
			hmRes.ProfileLast = true
		}
	}

	oldHint, gen, err := rollbackHint(surface, mach, preset, targetUser, profile)
	if err != nil {
		return guardedPlan{}, err
	}

	if hmRes.ProfileLast {
		hmRes.OldModern = probeHMGenerationModern(surface, mach, preset, targetUser, oldHint)
	}

	compositionTier := tier
	if hmRes.ProfileLast {
		compositionTier = installable.GuardTierStandard
	}

	// (5) Compose the transaction and the spawn argv (spec 6.5): the sweep,
	// the gc root, the boot-mode profile set and the OLD capture all run
	// inside start under its flock (spec 4.1 step 5), so the argv carries the
	// mutation targets instead of panix performing them.
	return composeSpawnPlan(mach, preset, targetUser, slotDir, tools, spawnComposition{
		newClosure:        newClosure,
		mode:              mode,
		tier:              compositionTier,
		userGate:          gate,
		profile:           profile,
		oldHint:           oldHint,
		gen:               gen,
		activationTimeout: activationTimeout,
		confirmTimeout:    confirmTimeout,
		hm:                hmRes,
	}), nil
}

// spawnComposition carries the deploy-level inputs the spawn window composes
// from (spec 6.5).
type spawnComposition struct {
	newClosure        string
	mode              string
	tier              installable.GuardTier
	userGate          ConfirmationGate
	profile           string
	oldHint           string
	gen               int64
	activationTimeout time.Duration
	confirmTimeout    time.Duration

	// hm carries the per-deploy home-manager composition resolution (zero
	// value for every other type and for legacy HM: the preset row composes
	// as today).
	hm hmComposition
}

// composeSpawnPlan composes the transaction and the spawn argv for one deploy
// (spec 4.2, 6.5): the step lists follow the preset's class and mode tables,
// the gate degrades to auto where the shape is not gate-worthy, and the
// mutation targets ride start's argv. The plan keeps only the identities the
// window needs (the lists live in the argv; the outcome paths read the
// records or inspect verdicts).
func composeSpawnPlan(
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir string,
	tools guardTools,
	comp spawnComposition,
) guardedPlan {
	steps := composeStepLists(preset, comp.mode, comp.profile, tools.nixEnv, comp.oldHint, comp.newClosure)
	if comp.hm.ProfileLast {
		// Modern home-manager composes profile-last (the darwin standard
		// shape) with the driver flag on OLD's revert activation when OLD's
		// generation supports it; the commit sets the resolved HM profile to
		// the bare activationPackage (the composed NEW itself).
		steps = composeHMStepLists(preset, comp.profile, tools.nixEnv, comp.oldHint, comp.newClosure, comp.hm.OldModern)
	}

	effectiveGate := EffectiveGate(comp.userGate, steps.GateEligible)
	gcRoot := gcRootTarget(comp.tier, comp.mode, comp.oldHint, comp.newClosure)

	bootSet := ""
	if comp.mode == "boot" {
		bootSet = comp.newClosure // the previously unimplemented boot-mode pre-start set (spec 5)
	}

	return guardedPlan{
		mach:              mach,
		preset:            preset,
		targetUser:        targetUser,
		slotDir:           slotDir,
		guardBin:          guardianBinaryPath(slotDir),
		key:               tools.key,
		gate:              effectiveGate,
		activationTimeout: comp.activationTimeout,
		confirmTimeout:    comp.confirmTimeout,
		spawnArgv: SpawnArgv(guardianBinaryPath(slotDir), slotDir, tools.key, comp.profile, tools.nixEnv, tools.nixStore,
			comp.newClosure, comp.gen, comp.mode, comp.tier, effectiveGate, comp.activationTimeout, comp.confirmTimeout,
			mach.RebootOnRevertFailure, mach.HealthChecksLocal,
			steps.Activation, steps.Commit, steps.Revert, steps.InvariantTarget,
			gcRoot, bootSet, true),
		profile: comp.profile,
		nixEnv:  tools.nixEnv,
	}
}

// runGuardedWindow runs the spawn exec as the duplex relay window (spec 8,
// 9.1): the exec is NOT quiet and NOT trimmed (this is the real-time
// narrative), its output tap feeds the wire stream, its stdin carries the
// command frames, and the magic-tier confirm loop drives the gate over the
// wire. The exit code maps to the phase outcome (spec 9.1).
func runGuardedWindow(exc *executioner.Executioner, plan guardedPlan) error {
	frameReader, frameWriter, err := os.Pipe()
	if err != nil {
		return errors.Wrap(err, "guard wire pipe")
	}

	stream := newWireStream(plan.key)

	startDone := make(chan struct{})
	if plan.gate == GateMagic {
		loop := newConfirmLoop(exc, plan, frameWriter, stream, startDone)
		go loop.run(exc.Context(), time.Now().Add(plan.activationTimeout+plan.confirmTimeout+confirmGiveUpSlack))
	}

	startedAt := time.Now()
	startErr := exc.Exec(
		"guarded activation",
		"activating (guarded)",
		"guarded activation failed",
		targetArgv(plan.mach, plan.preset, plan.targetUser, plan.spawnArgv),
		executioner.WithTimeout(spawnExecTimeout(plan.activationTimeout, plan.confirmTimeout)),
		executioner.WithStdin(frameReader),
		executioner.WithOutputTap(stream.feed),
	)

	close(startDone)

	// The pump unblocks through its reader's read deadline when the exec
	// ends; closing both ends releases the pipe either way.
	_ = frameReader.Close()
	_ = frameWriter.Close()

	return reportOutcome(exc, plan, startErr, startedAt)
}

// confirmLoop drives the magic-tier gate from the panix side (spec 2, 8, T4):
// the wire is the primary transport - records stream in through the spawn
// exec's output tap and the confirm/revert decisions ride the stdin frames -
// while the fresh-connection log tail stays as the floor when the wire is
// silent and the fresh-connection ctl stays as the ack fallback (the degraded
// path, spec 8). It owns a dedicated Executioner: the streaming spawn exec and
// this loop run concurrently and must not share one instance.
type confirmLoop struct {
	exc        *executioner.Executioner
	mach       *machine.Machine
	preset     installable.Preset
	targetUser string
	guardBin   string
	slotDir    string
	key        string
	checks     []string
	done       <-chan struct{}
	frames     io.Writer
	stream     *wireStream
	ctx        context.Context

	cursor     int   // stream high-water mark (loop-goroutine owned)
	rid        int64 // per-deploy monotonic frame counter
	checksDone bool
}

// newConfirmLoop clones the phase executioner for the loop's private use and
// binds the loop to the window's wire ends.
func newConfirmLoop(
	exc *executioner.Executioner,
	plan guardedPlan,
	frames io.Writer,
	stream *wireStream,
	done <-chan struct{},
) *confirmLoop {
	return &confirmLoop{
		exc:        exc.Clone(),
		mach:       plan.mach,
		preset:     plan.preset,
		targetUser: plan.targetUser,
		guardBin:   plan.guardBin,
		slotDir:    plan.slotDir,
		key:        plan.key,
		checks:     plan.mach.HealthChecks,
		done:       done,
		frames:     frames,
		stream:     stream,
	}
}

// run consumes the wire until the guardian reaches a terminal state, panix
// gives up (deadline: the guardian self-reverts at its own deadline, spec
// 6.2), the phase context is cancelled, or the spawn exec returns. Every
// drained record idempotently drives the gate; the floor poll keeps the gate
// moving when the wire stays silent.
func (l *confirmLoop) run(ctx context.Context, deadline time.Time) {
	l.ctx = ctx

	ticker := time.NewTicker(floorCheckInterval)
	defer ticker.Stop()

	lastWire := time.Now()
	lastFloor := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-l.done:
			return
		case <-ticker.C:
		case <-l.stream.notify:
		}

		if !time.Now().Before(deadline) {
			return
		}

		records, next := l.stream.drain(l.cursor)
		l.cursor = next

		if len(records) > 0 {
			lastWire = time.Now()
		}

		if !l.handleRecords(records) {
			return
		}

		if floorDue(lastWire, lastFloor) {
			lastFloor = time.Now()

			l.floorPoll()
		}
	}
}

// handleRecords drives the gate off one drained batch: terminal records stand
// the loop down (the spawn exec delivers the outcome), and the first activated
// record runs the checks-and-confirm decision exactly once. False stops the loop.
func (l *confirmLoop) handleRecords(records []streamRecord) bool {
	for _, rec := range records {
		if guard.IsTerminalEvent(rec.Ev) {
			return false
		}

		if !l.checksDone && l.activated(rec) {
			l.checksDone = true
			l.decide()
		}
	}

	return true
}

// floorDue reports whether the wire has been silent long enough for one fresh
// log-tail read (spec 3: fresh-connection log polling stays as the floor).
func floorDue(lastWire, lastFloor time.Time) bool {
	lastActivity := lastFloor
	if lastWire.After(lastFloor) {
		lastActivity = lastWire
	}

	return time.Since(lastActivity) > floorIdle
}

// activated reports whether the record carries the activated state: the
// ACTIVATED event itself or a STATE snapshot at the activated status.
func (l *confirmLoop) activated(rec streamRecord) bool {
	return rec.Ev == guard.EventActivated || rec.St == statusActivated
}

// decide runs the remote health checks once and delivers the verdict over the
// wire: confirm on success, revert-request on the first failing check (spec
// 2, T4). The checks stay loud fresh-connection execs; the frame rides the
// live wire behind the readiness gate (the activated record was observed
// first, so the remote PTY input queue is being drained); a missing wire ack
// within the window falls back to the fresh-connection ctl.
func (l *confirmLoop) decide() {
	ok := l.runRemoteChecks()

	verb := frameConfirm
	if !ok {
		verb = frameRevert
	}

	if l.stream.snapshot().Terminal {
		return // the transaction ended while the checks ran; the outcome stands
	}

	l.rid++
	l.writeFrame(verb, l.rid)

	if l.awaitAck(l.rid, verb == frameConfirm) {
		return
	}

	select {
	case <-l.done:
		return // the exec ended; the outcome mapping takes over from here
	default:
	}

	// Degraded fallback (spec 8): signals + tail-ack over a fresh connection;
	// the delivery failure is swallowed and the give-up bound ends the wait.
	ctlVerb := "revert-request"
	if verb == frameConfirm {
		ctlVerb = "confirm"
	}

	l.sendCtl(ctlVerb)
}

// awaitAck waits one ctlWait window for a record ack with the matching rid
// (spec 8: records without rid or with a mismatched rid are never acks).
func (l *confirmLoop) awaitAck(rid int64, confirm bool) bool {
	timer := time.NewTimer(ctlWait)
	defer timer.Stop()

	for {
		records, next := l.stream.drain(l.cursor)
		l.cursor = next

		for _, rec := range records {
			if isWireAck(rec, rid, confirm) {
				return true
			}
		}

		select {
		case <-l.stream.notify:
		case <-timer.C:
			return false
		case <-l.ctx.Done():
			return false
		case <-l.done:
			return false
		}
	}
}

// writeFrame delivers one command frame on the wire (spec 8). Best-effort
// transport: a write failure leaves the ack to the ctl fallback.
func (l *confirmLoop) writeFrame(command string, rid int64) {
	_, _ = io.WriteString(l.frames, formatFrame(command, rid))
}

// floorPoll reads the slot log tail over a fresh connection and feeds the
// records into the wire stream, so the loop's next drain sees them through the
// same idempotent path (spec 9.3: the log is the authority; the stream is
// display only).
func (l *confirmLoop) floorPoll() {
	tail, err := readLogTail(executionerSurface{ex: l.exc}, l.mach, l.preset, l.targetUser, l.slotDir, true)
	if err != nil {
		return // transient: the next idle window retries
	}

	l.stream.ingestTail(tail)
}

// runRemoteChecks executes the user's health checks on the target: loud,
// fresh-connection execs so each check renders in the narrative with its
// output; the first failure stops the sequence (the caller requests the
// revert, spec 2).
func (l *confirmLoop) runRemoteChecks() bool {
	for _, check := range l.checks {
		err := l.exc.Exec(
			"remote health check",
			"running health check",
			"health check failed: "+check,
			[]string{"/bin/sh", "-c", check},
			executioner.FreshConnection(),
			executioner.WithTimeout(remoteCheckTimeout),
		)
		if err != nil {
			return false
		}
	}

	return true
}

// sendCtl delivers a control request over a fresh connection (spec 8): the
// reachability proof and the delivery are the same exec. Errors are swallowed:
// the loop keeps polling and the give-up bound ends the wait. The ack-timeout
// exit (7) is the one distinct outcome: the request was delivered but no ack
// arrived within the wait, so it may still take effect, and the caveat lands
// in the narrative instead of vanishing with the exec.
func (l *confirmLoop) sendCtl(command string) {
	err := quietExec(executionerSurface{ex: l.exc}, "guard ctl "+command, "guard ctl "+command+" failed",
		targetArgv(l.mach, l.preset, l.targetUser, CtlArgv(l.guardBin, l.slotDir, l.key, command, ctlWait, "")),
		nil,
		executioner.FreshConnection(),
		executioner.WithTimeout(ctlExecTimeout),
	)

	if code, ok := exitCodeOfErr(err); !ok || code != guard.CtlExitAckTimedOut {
		return
	}

	_ = l.exc.ExecFn(
		"guard ctl "+command+" ack timeout",
		"guard ctl ack pending",
		"guard ctl ack timeout note failed",
		func(log *logs_command.CommandLog) error {
			log.Output.Write([]byte(fmt.Sprintf(
				"the %s request was delivered but no ack arrived within %s; it may still take effect",
				command, ctlWait)))

			return nil
		},
	)
}

// ExecuteGuardedActivation runs the guarded activation transaction (spec 4.1):
// the pre-start sequence (resolve, probes, legacy sweep, transfer, lock
// probe), the guardian spawn whose exec is the live relay and whose start verb
// performs every pre-start mutation under its flock (sweep, gc root, boot
// set, OLD capture), the magic-tier confirm loop over the wire, and the
// exit-code outcome mapping (spec 9.1, 9.4).
func ExecuteGuardedActivation(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	outputType, targetUser, newClosure, mode string,
) error {
	plan, err := prepareGuardedActivation(executionerSurface{ex: exc}, mach, preset,
		outputType, targetUser, newClosure, mode, GuardBinary)
	if err != nil {
		return err
	}

	return runGuardedWindow(exc, plan)
}

// resolveEmbedPath resolves the guardian embed for the target OS and
// architecture. Dry-run has no embed to miss: the preview composes against the
// canonical embed path instead of failing.
func resolveEmbedPath(surface execSurface, platform, arch string, embedResolver func(targetOS, targetArch string) (string, error)) (string, error) {
	normalized, err := NormalizeArch(arch)
	if err != nil {
		return "", err
	}

	if surface.dryRun() {
		return "embed/panix-guard-" + strings.ToLower(platform) + "-" + normalized + ".gz", nil
	}

	targetOS := "linux"
	if strings.EqualFold(platform, platformDarwin) {
		targetOS = "darwin"
	}

	return embedResolver(targetOS, normalized)
}

// reportOutcome maps the spawn exec's exit code to the phase outcome (spec
// 9.1, 9.5 start/attach block): 0 committed, 2 reverted, 3 revert_failed, 4
// failed_precondition, 5 activation-exited (minimal tier success), 6 cancelled
// (inline converge per 9.4); panix-side give-up reports the honest guard state
// without deciding. Every failure report's context comes from the records or
// the inspect verdict (spec 7), never from a panix-side capture.
func reportOutcome(
	exc *executioner.Executioner,
	plan guardedPlan,
	startErr error,
	startedAt time.Time,
) error {
	surface := executionerSurface{ex: exc}
	logPath := plan.slotDir + "/" + logName

	if startErr == nil {
		return nil // committed (spec 9.1)
	}

	if gaveUp, cause := panixGiveUpCause(exc, startErr, startedAt, spawnExecTimeout(
		plan.activationTimeout, plan.confirmTimeout)); gaveUp {
		return cancellationOutcome(surface, plan.mach, plan.preset, plan.targetUser,
			plan.slotDir, plan.key, startErr, cause)
	}

	code, ok := exitCodeOfErr(startErr)
	if !ok {
		// Transport failure mid-window (connection died during activation or the
		// confirm window): the guardian keeps running as a PID-1-owned process and
		// self-reverts or commits at the deadline. Reconnect, wait for the terminal
		// record, and report the honest outcome (spec 9.2: the log is the authority;
		// the stream is display only).
		activeSSH := plan.mach.GetActiveSSH()

		rerr := executioner.WaitForReconnect(exc, activeSSH,
			"waiting for machine after the connection was lost",
			"machine unreachable after disconnect; the guard self-reverts on its deadline")
		if rerr != nil {
			return errors.Errorf("guarded activation: machine unreachable after disconnect; the guard will self-revert (guard log: %s)", logPath)
		}

		return awaitTerminalOutcome(surface, plan.mach, plan.preset, plan.targetUser, plan.slotDir,
			plan.profile, plan.nixEnv, startedAt, spawnExecTimeout(plan.activationTimeout, plan.confirmTimeout))
	}

	switch code {
	case guard.ExitReverted:
		verdict, _, err := inspectSlot(surface, plan.mach, plan.preset, plan.targetUser,
			"guard slot inspect", guardianBinaryPath(plan.slotDir), plan.slotDir,
			executioner.FreshConnection())
		if err != nil || verdict.Err == "" {
			return errors.Errorf("activation failed: reverted to the previous generation (guard log: %s)", logPath)
		}

		return errors.Errorf("activation failed: %s (guard log: %s)", verdict.Err, logPath)
	case guard.ExitRevertFailed:
		return errors.Errorf("activation failed: the revert itself failed, operator action required (guard log: %s)", logPath)
	case guard.ExitFailedPrecondition:
		return errors.Errorf("activation failed: precondition check failed, zero effects (guard log: %s)", logPath)
	case guard.ExitActivationExited:
		return nil // minimal tier: the activation ran, no commit/revert applies (spec 9.1)
	case guard.ExitCancelled:
		return inlineDecideOutcome(surface, plan.mach, plan.preset, plan.targetUser, plan.slotDir,
			plan.profile, plan.nixEnv, plan.activationTimeout, startErr)
	default:
		return errors.Wrapf(startErr, "guarded activation ended with unknown outcome (exit %d, guard log: %s)", code, logPath)
	}
}

// verdictOutcome maps one terminal inspect verdict to the deploy outcome
// (spec 9.1's guardian outcome vocabulary, the same table reportOutcome
// applies to the spawn exec's exit code). The verdict's status carries the
// outcome - the terminal record's rc is the activation's own exit code (0 for
// every guardian-decided state), not the vocabulary - and err is the revert
// excerpt the guardian reported, when it reported one.
func verdictOutcome(verdict InspectVerdict, logPath string) error {
	switch verdict.Status {
	case statusCommitted, statusActivationExited:
		return nil // committed, or minimal tier: the activation ran and exited
	case statusReverted:
		if verdict.Err != "" {
			return errors.Errorf("activation failed: %s (guard log: %s)", verdict.Err, logPath)
		}

		return errors.Errorf("activation failed: reverted to the previous generation (guard log: %s)", logPath)
	case statusRevertFailed:
		return errors.Errorf("activation failed: the revert itself failed, operator action required (guard log: %s)", logPath)
	case statusFailedPrecondition:
		return errors.Errorf("activation failed: precondition check failed, zero effects (guard log: %s)", logPath)
	default:
		return errors.Errorf("guarded activation: outcome unknown (guard log: %s)", logPath)
	}
}

// awaitTerminalOutcome resolves the honest outcome after a mid-window disconnect
// (spec 9.2, 9.4): the machine reconnected, so poll short inspect one-shots over
// the master connection (cheap execs) until a terminal verdict appears or the
// transaction's deadline passes; a still-non-terminal state converges via inline
// converge and is reported from its records. The verdict's lock flag says the
// transaction is still live: a free lock with no terminal state means the
// guardian is dead (the lock outlives the whole transaction), so waiting for
// the deadline cannot produce a terminal record and the inline converge runs
// immediately.
func awaitTerminalOutcome(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir string,
	profile, nixEnv string,
	startedAt time.Time,
	bound time.Duration,
) error {
	logPath := slotDir + "/" + logName

	deadline := startedAt.Add(bound)

	var verdict InspectVerdict

	for time.Now().Before(deadline) {
		next, found, err := inspectSlot(surface, mach, preset, targetUser,
			"guard slot inspect", guardianBinaryPath(slotDir), slotDir,
			executioner.WithTimeout(inspectPollTimeout))
		if err != nil {
			return errors.Errorf("guarded activation: machine unreachable after disconnect; the guard will self-revert (guard log: %s)", logPath)
		}

		if !found {
			// The slot vanished mid-window (a reboot clears the tmpfs
			// system-tier slot): no durable state and no guardian left; the
			// next deploy's pre-start sweep resolves the outcome from the
			// generation evidence (spec 9.3 step 6). The probe wrapper folds
			// the absent inspect outcome (exit 4) into found=false.
			return errors.Errorf("guarded activation: the guard slot vanished mid-window; outcome unknown until the next deploy's sweep (guard log: %s)", logPath)
		}

		verdict = next

		if verdict.Terminal || !verdict.Lock {
			break
		}

		time.Sleep(2 * time.Second)
	}

	if !verdict.Terminal {
		// The guardian died mid-window without a terminal state: converge
		// offline and report the converged outcome as the deploy's own (spec
		// 9.4). inlineDecideOutcome resolves every branch - a failed converge
		// and an unreadable converged state report honestly, a mapped terminal
		// state is the deploy's outcome, not an unknown.
		return inlineDecideOutcome(surface, mach, preset, targetUser, slotDir, profile, nixEnv, bound, nil)
	}

	return verdictOutcome(verdict, logPath)
}

// panixGiveUpCause distinguishes panix-side give-up (user cancel, phase
// cancellation, exec timeout) from a guardian-delivered outcome: in those
// cases the guardian keeps the lock and converges autonomously (spec 4.1 step
// 3), so panix reports the honest state instead of deciding.
func panixGiveUpCause(exc *executioner.Executioner, startErr error, startedAt time.Time, total time.Duration) (bool, error) {
	switch {
	case exc.Context().Err() != nil || errors.Is(startErr, context.Canceled):
		return true, context.Canceled
	case errors.Is(startErr, context.DeadlineExceeded) || time.Since(startedAt) >= total:
		return true, context.DeadlineExceeded
	default:
		return false, nil
	}
}

// cancellationOutcome reports the honest guard state when panix gave up on
// the window: one best-effort fresh-connection give-up cancel (spec 8: ONE
// `ctl revert-request --cause cancel` before reporting, errors swallowed,
// late-ack after COMMIT_START, no-op when terminal), one inspect one-shot, no
// decide (the guardian continues and converges on its own), and a
// cancel-preserving error for the workflow. The cancel-vs-failed distinction
// lives in this outcome's report (spec 8): signals carry no payload and the
// log-append design was rejected, so no record can carry the cause.
func cancellationOutcome(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir, key string,
	startErr, cause error,
) error {
	logPath := slotDir + "/" + logName

	// The one best-effort give-up cancel (spec 8): the delivery failure is
	// swallowed, exactly like the confirm loop's degraded ctl fallback; the
	// ack-timeout exit (7) is fine to leave unobserved (the request may
	// still take effect and the guardian converges autonomously either way).
	_ = quietExec(surface, "guard ctl revert-request", "guard ctl revert-request failed",
		targetArgv(mach, preset, targetUser, CtlArgv(guardianBinaryPath(slotDir), slotDir, key, "revert-request", ctlWait, GuardCancelCause)),
		nil,
		executioner.FreshConnection(),
		executioner.WithTimeout(ctlExecTimeout),
	)

	status := "unknown"
	if verdict, found, err := inspectSlot(surface, mach, preset, targetUser,
		"guard slot inspect", guardianBinaryPath(slotDir), slotDir,
		executioner.FreshConnection()); err == nil && found {
		status = statusOrUnknown(verdict.Status)
	}

	message := fmt.Sprintf("activation cancelled; the guardian converges autonomously; last guard status: %s (guard log: %s)", status, logPath)

	if !errors.Is(startErr, cause) {
		startErr = cause
	}

	return errors.Wrap(startErr, message)
}

// inlineDecideOutcome converges a dead guardian's slot inline (spec 9.4) and
// reports the converged outcome as the deploy's own. Converge is
// self-sufficient: it classifies from the log tail and converges from the
// transaction's own TXN record lists (argv fallback and generation arithmetic
// behind it), so panix passes no step lists and no recovered identity beyond
// the profile path and the resolved nix-env (the context fallback the records
// cannot fully supply: the fold restores the profile when the records carry
// it, nix-env exists only here). The
// post-mortem runs WITHOUT --truncate (spec 9.3: truncation is the pre-start
// caller's job; the inline post-mortem keeps the log for reporting - and a
// truncated log would blind the read-back below). Converge prints no summary,
// so the converged outcome is read back through a follow-up inspect one-shot:
// the terminal records converge writes are the durable result, and inspect is
// the one exec that folds them.
func inlineDecideOutcome(
	surface execSurface,
	mach *machine.Machine,
	preset installable.Preset,
	targetUser, slotDir string,
	profile, nixEnv string,
	convergeTimeout time.Duration,
	startErr error,
) error {
	logPath := slotDir + "/" + logName

	_, convergeErr := surface.run(
		"guard post-mortem convergence",
		"converging the interrupted guard transaction",
		"guard post-mortem convergence failed",
		targetArgv(mach, preset, targetUser, ConvergeArgv(guardianBinaryPath(slotDir), slotDir, profile, nixEnv, false)),
		executioner.FreshConnection(),
		executioner.WithTimeout(convergeTimeout),
	)
	if convergeErr != nil {
		if code, ok := exitCodeOfErr(convergeErr); ok {
			switch code {
			case guard.ConvergeExitLocked:
				// A live transaction still holds the lock; unreachable after a
				// relay EOF in practice, so report it honestly and stop.
				return errors.Wrapf(convergeErr, "activation failed: the guard slot is still locked by a live transaction (guard log: %s)", logPath)
			case guard.ConvergeExitFailed:
				return errors.Wrap(convergeErr, "activation failed: the guardian died mid-deploy and post-mortem convergence failed")
			}
		}

		return errors.Wrap(convergeErr, "activation failed: the guardian died mid-deploy and post-mortem convergence failed")
	}

	after, found, err := inspectSlot(surface, mach, preset, targetUser,
		"guard slot inspect", guardianBinaryPath(slotDir), slotDir,
		executioner.FreshConnection())
	if err != nil || !found {
		if startErr == nil {
			// The post-disconnect poll path carries no window error; the
			// unknown outcome must still report (pkg/errors.Wrap would drop it).
			return errors.Errorf("activation outcome unknown (guard log: %s): converged state unreadable", logPath)
		}

		return errors.Wrapf(startErr, "activation outcome unknown (guard log: %s): converged state unreadable", logPath)
	}

	outcome := verdictOutcome(after, logPath)
	if outcome != nil {
		return outcome
	}

	return nil // the deploy actually completed (or the activation ran and exited)
}
