package guard

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/guard"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGuardMachine builds a LOCAL machine with a controlled runtime IsRoot, a
// probed architecture and the inspect-era generation inventory (the rollback
// hint's source), so recorded argv shows privilege prefixes directly.
func newGuardMachine(t *testing.T, isRoot bool, rollback string) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.Rollback = attributes.Rollback(rollback)
	mach.MetaInspect.Store(&machine.MetaInspect{
		IsRoot:       isRoot,
		Architecture: "x86_64",
		Generations:  &machine.Generations{Current: 1, Available: []uint{1}},
	})

	return mach
}

//go:fix inline
func boolPtr(b bool) *bool { return new(b) }

// guardPreset is the nixos preset row from the single source of truth
// (internal/config/tree/installable/presets.go): the guarded path composes
// tier, profile and activation paths from it.
func guardPreset(t *testing.T) installable.Preset {
	t.Helper()

	preset, ok := installable.PresetForType(installable.FlakeOutputType("nixosConfigurations"))
	require.True(t, ok, "nixosConfigurations preset missing from the preset table")

	return preset
}

// TestGuardBinaryFailsWithoutEmbed pins the T7 build contract: guarded deploys
// fail loudly instead of transferring a missing binary.
func TestGuardBinaryFailsWithoutEmbed(t *testing.T) {
	t.Parallel()

	_, err := guardBinary("x86_64")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "guardian embed not built (T7)")
	assert.Contains(t, err.Error(), "x86_64")
}

// TestExecuteGuardedActivation_DryRunSpawnArgv drives the full pre-start
// sequence in dry-run and pins the streaming spawn exec's argv: the guardian
// receives the composed transaction for the tier and mode, wrapped with the
// machine's privilege model.
func TestExecuteGuardedActivation_DryRunSpawnArgv(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, false, "auto")
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	err := ExecuteGuardedActivation(exc, mach, guardPreset(t), "nixosConfigurations", "", "/nix/store/new-closure", "switch")
	require.NoError(t, err)

	lines := testutil.CommandLines(t, phaseLog)
	require.Len(t, lines, 2, "the transfer (loud) and the spawn (loud) register; pre-start steps stay quiet")

	assert.Equal(t, "cat embed/panix-guard-linux-amd64.gz", lines[0], "the transfer source streams the materialized guardian")

	spawn := lines[1]
	slot := SlotDir("/nix/var/nix/profiles/system", true, false)
	assert.True(t, strings.HasPrefix(spawn, "sudo "+slot+"/panix-guard start"), "sudo-wrapped spawn: %s", spawn)
	assert.Contains(t, spawn, "--dir "+slot)
	assert.Contains(t, spawn, "--tier full")
	assert.Contains(t, spawn, "--confirmation auto", "auto tier runs auto even when the shape is gate-worthy")
	assert.Contains(t, spawn, "--mode switch")
	assert.Contains(t, spawn, "--gen 1", "the generation comes from the inspect-era inventory")
	assert.NotContains(t, spawn, " --old ", "start captures OLD post-converge; the flag left the spawn argv")
	assert.Contains(t, spawn, "--new /nix/store/new-closure")
	assert.Contains(t, spawn, "--sweep=true", "the sweep runs under start's flock")
	assert.Contains(t, spawn, "--gc-root-target /nix/store/new-closure", "switch roots NEW (spec 4.3)")
	assert.NotContains(t, spawn, "--boot-set", "the boot set rides only boot mode")
	assert.Contains(t, spawn, "--nix-store nix-store", "dry-run resolves the tool to its bare name")
	assert.Contains(t, spawn, "--activation-argv [[\"/nix/store/new-closure/bin/switch-to-configuration\",\"test\"]]")
	assert.Contains(t, spawn, "--commit-argv [[\"nix-env\",\"-p\",\"/nix/var/nix/profiles/system\",\"--set\",\"/nix/store/new-closure\"],[\"/nix/store/new-closure/bin/switch-to-configuration\",\"boot\"]]")
	assert.Contains(t, spawn, "--revert-argv [[\"/nix/store/dry-run-old-closure/bin/switch-to-configuration\",\"switch\"]]")
	assert.Contains(t, spawn, "--invariant-target /nix/store/new-closure")
	assert.Contains(t, spawn, "--health-checks-local []")
}

// TestExecuteGuardedActivation_DryRunMagicGate pins the effective gate: the
// user's magic tier survives composition only where the transaction is
// gate-worthy, and the confirm loop is armed for it.
func TestExecuteGuardedActivation_DryRunMagicGate(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, false, "magic")
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	err := ExecuteGuardedActivation(exc, mach, guardPreset(t), "nixosConfigurations", "", "/nix/store/new-closure", "switch")
	require.NoError(t, err)

	assert.Contains(t, testutil.LastCommandLine(t, phaseLog), "--confirmation magic")

	t.Run("test mode is not gate-worthy", func(t *testing.T) {
		t.Parallel()

		exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

		err := ExecuteGuardedActivation(exc, mach, guardPreset(t), "nixosConfigurations", "", "/nix/store/new-closure", "test")
		require.NoError(t, err)

		assert.Contains(t, testutil.LastCommandLine(t, phaseLog), "--confirmation auto")
	})
}

// TestExecuteGuardedActivation_DryRunUserTier pins the user-level wrap: a
// home-manager deploy spawns through su -l with the user-state slot.
func TestExecuteGuardedActivation_DryRunUserTier(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, false, "auto")
	mach.Rollback = "auto"
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	preset, ok := installable.PresetForType(installable.FlakeOutputType("homeConfigurations"))
	require.True(t, ok, "homeConfigurations preset missing from the preset table")

	err := ExecuteGuardedActivation(exc, mach, preset, "homeConfigurations", "alice", "/nix/store/new-hm", "switch")
	require.NoError(t, err)

	spawn := testutil.LastCommandLine(t, phaseLog)
	slot := SlotDir("~/.local/state/nix/profiles/home-manager", false, false)

	// su -l single-quotes each argv element (shellquote.QuoteWord); strip the
	// quotes so the assertions read like the unwrapped argv.
	unwrapped := strings.ReplaceAll(spawn, "'", "")

	assert.True(t, strings.HasPrefix(spawn, "su -l alice -c "), "user-level spawn wraps as the target user: %s", spawn)
	assert.Contains(t, unwrapped, slot+"/panix-guard start")
	assert.Contains(t, unwrapped, "--tier self-setting")
	assert.Contains(t, unwrapped, "--invariant-target /nix/store/new-hm")
	assert.Contains(t, unwrapped, "--gc-root-target /nix/store/dry-run-old-closure", "self-setting roots OLD (spec 4.3)")
	assert.NotContains(t, unwrapped, "--boot-set")
}

// TestExecuteGuardedActivation_DryRunBootMode pins the boot-mode composition
// through the full pre-start flow: the previously unimplemented pre-start
// profile set rides start's argv (spec 5) and the gc root protects OLD, since
// the profile is re-pointed under start's lock before the activation runs.
func TestExecuteGuardedActivation_DryRunBootMode(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, false, "auto")
	exc, phaseLog := testutil.NewDryRunExecutioner(t, mach, phase.Activate)

	err := ExecuteGuardedActivation(exc, mach, guardPreset(t), "nixosConfigurations", "", "/nix/store/new-closure", "boot")
	require.NoError(t, err)

	spawn := testutil.LastCommandLine(t, phaseLog)

	assert.Contains(t, spawn, "--mode boot")
	assert.Contains(t, spawn, "--boot-set /nix/store/new-closure")
	assert.Contains(t, spawn, "--gc-root-target /nix/store/dry-run-old-closure", "boot mode roots OLD (spec 4.3)")
	assert.Contains(t, spawn, "--activation-argv [[\"/nix/store/new-closure/bin/switch-to-configuration\",\"boot\"]]")
	assert.Contains(t, spawn, "--commit-argv []", "boot mode has no commit step")
}

// scriptedSurface records every exec in order and answers from a per-step
// script keyed by the step description, so the pre-start sequence is
// observable and deterministic without a transport.
type scriptedSurface struct {
	mu       sync.Mutex
	calls    []string
	outcomes map[string][]fakeStepOutcome
	dryRunOn bool
}

// fakeStepOutcome is one scripted answer: captured stdout and the error to
// return (a synthesized exec error for exit-code probes).
type fakeStepOutcome struct {
	output string
	err    error
}

// TestInspectSlotParsesVerdictOnUnparseableExit pins the deployer half of the
// inspect contract (spec 9.5): exit 1 means the guardian still printed a valid
// verdict (legacy or unparseable log) with a truthful lock state, so the
// deployer must parse it instead of failing closed. Regression: the legacy
// sweep gate leg failed because exit 1 dropped the verdict.
func TestInspectSlotParsesVerdictOnUnparseableExit(t *testing.T) {
	t.Parallel()

	unparseableExit := exec.Command("false").Run()

	t.Run("exit 1 with verdict parses", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.script("guard slot lock probe", fakeStepOutcome{
			output: `{"status":"legacy","terminal":false,"lock":false}`,
			err:    unparseableExit,
		})

		verdict, found, err := inspectSlot(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
			"guard slot lock probe", "/run/panix-guard/v2/x/panix-guard", "/run/panix-guard/v2/x")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "legacy", verdict.Status)
		assert.False(t, verdict.Lock)
	})

	t.Run("exit 1 with locked verdict refuses the deploy", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.script("guard slot lock probe", fakeStepOutcome{
			output: `{"status":"legacy","terminal":false,"lock":true}`,
			err:    unparseableExit,
		})

		err := probeSlotLock(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "", "/run/panix-guard/v2/x")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "live deploy")
	})

	t.Run("exit 1 without a verdict fails closed", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.script("guard slot lock probe", fakeStepOutcome{err: unparseableExit})

		_, found, err := inspectSlot(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
			"guard slot lock probe", "/run/panix-guard/v2/x/panix-guard", "/run/panix-guard/v2/x")
		require.Error(t, err)
		assert.False(t, found)
	})
}

// TestInspectSlotAbsentMarkerIsQuiet pins the probe wrapper's demotion of the
// expected-clean absent outcome (inspect exit 4): the marker reads as the
// absent verdict with no error, so fresh slots and absent legacy dirs stop
// logging ERR into the phase logs. A genuine failure keeps erroring (the exit
// 1 subtests above stay loud).
func TestInspectSlotAbsentMarkerIsQuiet(t *testing.T) {
	t.Parallel()

	surface := newScriptedSurface()
	surface.script("guard slot lock probe", fakeStepOutcome{output: "absent\n"})

	verdict, found, err := inspectSlot(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
		"guard slot lock probe", "/run/panix-guard/v2/x/panix-guard", "/run/panix-guard/v2/x")
	require.NoError(t, err, "the absent outcome must not fail the probe")
	assert.False(t, found)
	assert.Equal(t, InspectVerdict{}, verdict)
}

func newScriptedSurface() *scriptedSurface {
	return &scriptedSurface{outcomes: map[string][]fakeStepOutcome{}}
}

// script queues the outcomes for one description; they are consumed in order
// and the last one repeats (multi-read steps see sequenced tails).
func (s *scriptedSurface) script(description string, outcomes ...fakeStepOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.outcomes[description] = outcomes
}

func (s *scriptedSurface) next(description string) fakeStepOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue := s.outcomes[description]
	if len(queue) == 0 {
		return fakeStepOutcome{}
	}

	out := queue[0]
	if len(queue) > 1 {
		s.outcomes[description] = queue[1:]
	}

	return out
}

func (s *scriptedSurface) run(
	description, _, _ string,
	argv []string,
	_ ...executioner.ExecOption,
) (string, error) {
	s.mu.Lock()
	s.calls = append(s.calls, description+" :: "+strings.Join(argv, " "))
	s.mu.Unlock()

	out := s.next(description)

	return strings.TrimSpace(out.output), out.err
}

func (s *scriptedSurface) runPipe(
	description, _, _ string,
	spec executioner.PipeSpec,
	_ ...executioner.ExecOption,
) error {
	s.mu.Lock()
	s.calls = append(s.calls, description+" :: "+strings.Join(spec.Source, " "))
	s.mu.Unlock()

	return s.next(description).err
}

func (s *scriptedSurface) dryRun() bool { return s.dryRunOn }

// stepCalls snapshots the recorded call sequence (description :: argv lines).
func (s *scriptedSurface) stepCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.calls)
}

// callIndex returns the position of the first call with the given description
// prefix, or -1 when the step never ran.
func callIndex(calls []string, description string) int {
	for i, call := range calls {
		if strings.HasPrefix(call, description+" ::") {
			return i
		}
	}

	return -1
}

// stubEmbed stands in for the guardian embed: the real embed ships only in
// flake builds (spec 15), so the scripted-surface tests resolve a fake path.
func stubEmbed(targetOS, targetArch string) (string, error) {
	return "embed/panix-guard-" + targetOS + "-" + targetArch + ".gz", nil
}

// TestPrepareGuardedActivation_PreStartOrder pins the reordered pre-start
// sequence (spec 4.1): resolution and probes first, the legacy sweep ahead of
// the transfer, the transfer ahead of the lock probe (so the probe always has
// a current binary), and every v1.3 pre-start mutation (capture, gc root,
// sweep) gone from panix's sequence - they ride start's argv under its flock.
func TestPrepareGuardedActivation_PreStartOrder(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, true, "auto")
	surface := newScriptedSurface()
	surface.script("detect platform", fakeStepOutcome{output: "Linux\n"})
	surface.script("resolve nix-env", fakeStepOutcome{output: testNixEnv + "\n"})
	surface.script("resolve nix-store", fakeStepOutcome{output: testNixStore + "\n"})
	surface.script("resolve busctl", fakeStepOutcome{output: "/run/current-system/sw/bin/busctl\n"})
	surface.script("resolve rollback target", fakeStepOutcome{output: "/nix/store/hint-old\n"})
	surface.script("KillUserProcesses probe", fakeStepOutcome{output: "KillUserProcesses=no\n"})
	// The legacy slot dir exists and its inspect verdict reports a free lock:
	// the tree is swept away.
	surface.script("legacy guard slot probe", fakeStepOutcome{output: "yes\n"})
	surface.script("legacy guard slot lock probe", fakeStepOutcome{output: `{"status":"legacy","terminal":false,"lock":false}` + "\n"})
	surface.script("legacy guard slot cleanup", fakeStepOutcome{})
	// The v2 slot's inspect verdict reports a free lock: the probe passes.
	surface.script("guard slot lock probe", fakeStepOutcome{output: `{"status":"unknown","terminal":false,"lock":false}` + "\n"})

	plan, err := prepareGuardedActivation(surface, mach, guardPreset(t), "nixosConfigurations", "", "/nix/store/new-closure", "switch", stubEmbed)
	require.NoError(t, err)

	calls := surface.stepCalls()

	kup := callIndex(calls, "KillUserProcesses probe")
	legacyCleanup := callIndex(calls, "legacy guard slot cleanup")
	transfer := callIndex(calls, "transfer guardian")
	lockProbe := callIndex(calls, "guard slot lock probe")

	require.Greater(t, kup, -1)
	require.Greater(t, legacyCleanup, -1)
	require.Greater(t, transfer, -1)
	require.Greater(t, lockProbe, -1)

	assert.Less(t, kup, transfer, "the KUP probe precedes the transfer")
	assert.Less(t, transfer, legacyCleanup, "the inspect-based legacy probe runs the transferred v2 binary (spec 4.1 steps 2-3)")
	assert.Less(t, legacyCleanup, lockProbe, "the legacy sweep precedes the v2 lock probe")

	// The lock probes are inspect one-shots now (wrapped in the probe script
	// that demotes the absent outcome): the legacy verdict targets the legacy
	// directory through the v2 binary, and the v2 probe targets the slot.
	legacyInspect := calls[callIndex(calls, "legacy guard slot lock probe")]
	assert.Contains(t, legacyInspect, "'inspect' '--dir'", "the lock probes are inspect one-shots")
	assert.Contains(t, legacyInspect, "'--dir' '"+LegacySlotDir(plan.slotDir)+"'", "the legacy inspect targets the legacy directory")
	assert.Contains(t, calls[lockProbe], "'--dir' '"+plan.slotDir+"'")

	// The expected-clean existence probe never fails the exec on absence: the
	// shell reports the verdict on stdout and always exits 0, so fresh slots
	// stop screaming ERR into the phase logs.
	legacyDirProbe := calls[callIndex(calls, "legacy guard slot probe")]
	assert.Contains(t, legacyDirProbe, "if [ -e", "the existence probe reports absence on stdout")
	assert.Contains(t, legacyDirProbe, "printf no", "absence must not fail the exec")
	assert.NotContains(t, legacyDirProbe, "test -e", "the bare test(1) probe logged ERR for the expected-clean case")

	// The v1.3 pre-start mutations left the panix sequence entirely: they run
	// inside start under its flock (spec 4.1 step 5). This is also the
	// structural mode-drift regression: panix no longer recomposes the
	// previous transaction's step lists anywhere.
	joined := strings.Join(calls, "\n")
	for _, gone := range []string{
		"capture rollback target",
		"capture generation number",
		"guard gc root",
		"guard slot sweep",
		"guard sweep convergence",
	} {
		assert.NotContains(t, joined, gone+" ::", "%s must not run panix-side anymore", gone)
	}

	// The spawn argv carries the mutations instead.
	spawn := strings.Join(plan.spawnArgv, " ")
	assert.Contains(t, spawn, "--sweep=true")
	assert.Contains(t, spawn, "--gc-root-target /nix/store/new-closure", "switch roots NEW (spec 4.3)")
	assert.NotContains(t, spawn, "--boot-set", "the boot set rides only boot mode")
	assert.Contains(t, spawn, "--nix-store "+testNixStore)
	assert.NotContains(t, spawn, " --old", "start captures OLD post-converge")
	assert.Contains(t, spawn, "--gen 1", "the generation comes from the inspect-era inventory")
	assert.Contains(t, spawn, "--revert-argv [[\"/nix/store/hint-old/bin/switch-to-configuration\",\"switch\"]]",
		"the argv lists compose against the rollback-target hint")
}

// TestPrepareGuardedActivation_BootModeArgv pins the boot-mode composition:
// the previously unimplemented pre-start profile set rides start's argv and
// the gc root protects OLD (the profile is re-pointed at pre-start).
func TestPrepareGuardedActivation_BootModeArgv(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, true, "auto")
	surface := newScriptedSurface()
	surface.script("detect platform", fakeStepOutcome{output: "Linux\n"})
	surface.script("resolve nix-env", fakeStepOutcome{output: testNixEnv + "\n"})
	surface.script("resolve nix-store", fakeStepOutcome{output: testNixStore + "\n"})
	surface.script("resolve busctl", fakeStepOutcome{output: "/run/current-system/sw/bin/busctl\n"})
	surface.script("resolve rollback target", fakeStepOutcome{output: "/nix/store/hint-old\n"})
	surface.script("KillUserProcesses probe", fakeStepOutcome{output: "KillUserProcesses=no\n"})
	// No legacy slot and a fresh v2 slot (absence folded into the probe
	// wrapper's stdout marker, so the missing slot reads as free and quiet).
	surface.script("legacy guard slot probe", fakeStepOutcome{output: "no\n"})
	surface.script("legacy guard slot lock probe", fakeStepOutcome{output: "absent"})
	surface.script("guard slot lock probe", fakeStepOutcome{output: "absent"})

	plan, err := prepareGuardedActivation(surface, mach, guardPreset(t), "nixosConfigurations", "", "/nix/store/new-closure", "boot", stubEmbed)
	require.NoError(t, err)

	spawn := strings.Join(plan.spawnArgv, " ")
	assert.Contains(t, spawn, "--boot-set /nix/store/new-closure", "boot mode sets NEW at pre-start, under the lock")
	assert.Contains(t, spawn, "--gc-root-target /nix/store/hint-old", "boot mode roots OLD (spec 4.3)")
	assert.Contains(t, spawn, "--activation-argv [[\"/nix/store/new-closure/bin/switch-to-configuration\",\"boot\"]]")
}

// homePresetScript builds a scripted pre-start sequence for a home-manager
// installable: the shared resolution/probe scripts plus the caller's
// gen-version and profile-probe answers.
func newHMPrestartSurface(t *testing.T, genVersionNew, genVersionOld, resolvedProfile string) *scriptedSurface {
	t.Helper()

	surface := newScriptedSurface()
	surface.script("detect platform", fakeStepOutcome{output: "Linux\n"})
	surface.script("resolve nix-env", fakeStepOutcome{output: testNixEnv + "\n"})
	surface.script("resolve nix-store", fakeStepOutcome{output: testNixStore + "\n"})
	surface.script("resolve busctl", fakeStepOutcome{output: "/run/current-system/sw/bin/busctl\n"})
	surface.script("resolve rollback target", fakeStepOutcome{output: "/nix/store/hint-old\n"})
	surface.script("KillUserProcesses probe", fakeStepOutcome{output: "KillUserProcesses=no\n"})
	surface.script("legacy guard slot probe", fakeStepOutcome{output: "no\n"})
	surface.script("legacy guard slot lock probe", fakeStepOutcome{output: "absent"})
	surface.script("guard slot lock probe", fakeStepOutcome{output: "absent"})

	if genVersionNew != "" {
		surface.script("probe home-manager driver support", fakeStepOutcome{output: genVersionNew + "\n"})
	}

	if resolvedProfile != "" {
		surface.script("resolve the home-manager profile", fakeStepOutcome{output: resolvedProfile})
	}

	// The OLD probe consumes a second scripted answer for the same
	// description (the queue advances per read).
	if genVersionOld != "" {
		surface.script("probe home-manager driver support",
			fakeStepOutcome{output: genVersionNew + "\n"}, fakeStepOutcome{output: genVersionOld + "\n"})
	}

	return surface
}

// TestPrepareGuardedActivation_HMProfileLast pins the modern home-manager
// composition (HM >= 25.11, spec 4.2 tier table): the per-deploy detection
// probes run read-only before the rollback hint, the profile-last branch
// composes the driver-1 activation and the --set commit against the
// HM-rule-resolved profile, and the transaction's tier reports standard.
func TestPrepareGuardedActivation_HMProfileLast(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, false, "auto")
	surface := newHMPrestartSurface(t, "1", "1", "/home/hmuser/.local/state/nix/profiles/home-manager")

	plan, err := prepareGuardedActivation(surface, mach, presetOf(t, "homeConfigurations"),
		"homeConfigurations", "hmuser", "/nix/store/hm-new", "switch", stubEmbed)
	require.NoError(t, err)

	calls := surface.stepCalls()

	probeIdx := callIndex(calls, "probe home-manager driver support")
	resolveIdx := callIndex(calls, "resolve the home-manager profile")
	hintIdx := callIndex(calls, "resolve rollback target")

	require.Greater(t, probeIdx, -1)
	require.Greater(t, resolveIdx, -1)
	require.Greater(t, hintIdx, -1)

	assert.Less(t, probeIdx, resolveIdx, "the NEW driver probe precedes the profile resolution")
	assert.Less(t, resolveIdx, hintIdx, "the profile resolution precedes the rollback hint")

	// The OLD probe reuses the same description: exactly two probes run, and
	// the second (OLD) follows the hint.
	probes := 0

	secondProbe := -1

	for i, call := range calls {
		if strings.HasPrefix(call, "probe home-manager driver support") {
			probes++

			if probes == 2 {
				secondProbe = i
			}
		}
	}

	assert.Equal(t, 2, probes, "the modern deploy probes NEW and OLD")
	assert.Greater(t, secondProbe, hintIdx, "the OLD driver probe follows the hint")

	spawn := strings.Join(plan.spawnArgv, " ")

	assert.Contains(t, spawn, "--tier standard", "modern HM composes profile-last (the standard tier)")
	assert.Contains(t, spawn,
		`--activation-argv [["/nix/store/hm-new/activate","--driver-version","1"]]`,
		"the driver-1 activation rides the composed activation child")
	assert.Contains(t, spawn,
		`--commit-argv [["`+testNixEnv+`","-p","/home/hmuser/.local/state/nix/profiles/home-manager","--set","/nix/store/hm-new"]]`,
		"the commit sets the resolved HM profile to the bare activationPackage")
	assert.Contains(t, spawn,
		`--revert-argv [["/nix/store/hint-old/activate","--driver-version","1"]]`,
		"OLD modern carries the driver flag on the revert activation")
	assert.Contains(t, spawn, "--gc-root-target /nix/store/hm-new", "profile-last roots NEW (spec 4.3)")
	assert.Contains(t, spawn, "--invariant-target /nix/store/hm-new")
	assert.Contains(t, spawn, "--profile /home/hmuser/.local/state/nix/profiles/home-manager",
		"the guardian's runtime profile rules target the resolved HM profile")
	assert.Contains(t, spawn, "--builtin-unit-check=false", "HM runs no unit baseline")
}

// TestPrepareGuardedActivation_HMProfileResolution pins the HM-rule profile
// resolution script: state-home profiles dir wins when it exists, the
// per-user nix store profile is the fallback, and the parent dir is created
// before first use (mkdir -p).
func TestPrepareGuardedActivation_HMProfileResolve(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, false, "auto")
	surface := newHMPrestartSurface(t, "1", "1", "/home/hmuser/.local/state/nix/profiles/home-manager")

	_, err := prepareGuardedActivation(surface, mach, presetOf(t, "homeConfigurations"),
		"homeConfigurations", "hmuser", "/nix/store/hm-new", "switch", stubEmbed)
	require.NoError(t, err)

	resolveCall := ""

	for _, call := range surface.stepCalls() {
		if strings.HasPrefix(call, "resolve the home-manager profile") {
			resolveCall = call
		}
	}

	require.NotEmpty(t, resolveCall)

	assert.Contains(t, resolveCall, "'sh' '-c'", "the resolution runs one shell probe")
	assert.Contains(t, resolveCall, "XDG_STATE_HOME:-$HOME/.local/state", "the state-home profiles rule")
	assert.Contains(t, resolveCall, "mkdir -p", "the parent profiles dir is created before first use")
	assert.Contains(t, resolveCall, "NIX_STATE_DIR:-/nix/var/nix}/profiles/per-user", "the per-user fallback rule")
	assert.Contains(t, resolveCall, "su -l hmuser", "the probe runs as the deploy identity")
}

// TestPrepareGuardedActivation_HMLegacyKeepsSelfSetting pins the no-regression
// path: a legacy HM generation (gen-version missing, unparseable, or zero)
// keeps today's self-setting composition byte-identically: no commit step,
// OLD roots the gc root, bare activate steps, and the transaction's profile
// stays the preset's static path.
func TestPrepareGuardedActivation_HMLegacyKeepsSelfSetting(t *testing.T) {
	t.Parallel()

	for name, genVersion := range map[string]string{
		"missing gen-version": "",
		"gen-version zero":    "0",
		"unparseable content": "abc",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mach := newGuardMachine(t, false, "auto")
			surface := newHMPrestartSurface(t, genVersion, "", "")

			plan, err := prepareGuardedActivation(surface, mach, presetOf(t, "homeConfigurations"),
				"homeConfigurations", "hmuser", "/nix/store/hm-new", "switch", stubEmbed)
			require.NoError(t, err)

			calls := surface.stepCalls()
			assert.Equal(t, -1, callIndex(calls, "resolve the home-manager profile"),
				"the legacy path resolves no HM-rule profile")

			// Exactly one probe runs (NEW); the OLD probe is skipped
			// entirely: the legacy composition never consults OLD's driver.
			probes := 0

			for _, call := range calls {
				if strings.HasPrefix(call, "probe home-manager driver support") {
					probes++
				}
			}

			assert.Equal(t, 1, probes, "a legacy NEW skips the OLD probe")

			spawn := strings.Join(plan.spawnArgv, " ")

			assert.Contains(t, spawn, "--tier self-setting", "the legacy composition tier is unchanged")
			assert.Contains(t, spawn, `--activation-argv [["/nix/store/hm-new/activate"]]`, "bare activation child")
			assert.Contains(t, spawn, "--commit-argv []", "self-setting has no commit step")
			assert.Contains(t, spawn, `--revert-argv [["/nix/store/hint-old/activate"]]`, "bare revert activation")
			assert.Contains(t, spawn, "--gc-root-target /nix/store/hint-old", "self-setting roots OLD (spec 4.3)")
			assert.NotContains(t, spawn, "--driver-version", "no driver flag in any legacy argv")
		})
	}
}

// TestProbeHMGenerationModern pins the detection helper against the scripted
// surface: integer >= 1 is modern; zero, unreadable, and unparseable are
// legacy; dry-run never probes and reports legacy (previews compose the
// legacy shape).
func TestProbeHMGenerationModern(t *testing.T) {
	t.Parallel()

	probe := func(t *testing.T, outcome fakeStepOutcome) bool {
		t.Helper()

		surface := newScriptedSurface()
		surface.script("probe home-manager driver support", outcome)

		return probeHMGenerationModern(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
			"/nix/store/new")
	}

	assert.True(t, probe(t, fakeStepOutcome{output: "1\n"}), "gen-version 1 is modern")
	assert.True(t, probe(t, fakeStepOutcome{output: "7"}), "higher integers are modern")
	assert.False(t, probe(t, fakeStepOutcome{output: "0\n"}), "gen-version 0 is legacy")
	assert.False(t, probe(t, fakeStepOutcome{output: "soon"}), "unparseable is legacy")
	assert.False(t, probe(t, fakeStepOutcome{err: exitErr(t, 1)}), "unreadable is legacy")
	assert.False(t, probe(t, fakeStepOutcome{}), "empty output is legacy")
}

// TestRollbackHint pins the argv-composition rollback target: the closure of
// the inspected current generation, the dry-run placeholder, and the
// fail-closed paths.
func TestRollbackHint(t *testing.T) {
	t.Parallel()

	mach := newGuardMachine(t, true, "auto")
	mach.MetaInspect.Store(&machine.MetaInspect{
		IsRoot:       true,
		Architecture: "x86_64",
		Generations:  &machine.Generations{Current: 2, Available: []uint{1, 2}},
	})

	t.Run("real run resolves the inspected generation's closure", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.script("resolve rollback target", fakeStepOutcome{output: "/nix/store/gen2\n"})

		oldClosure, gen, err := rollbackHint(surface, mach, guardPreset(t), "", testProfile)
		require.NoError(t, err)

		assert.Equal(t, "/nix/store/gen2", oldClosure)
		assert.Equal(t, int64(2), gen)
		assert.Contains(t, surface.stepCalls()[0], testProfile+"-2-link", "the hint reads the generation link, not the profile")
	})

	t.Run("dry-run composes the placeholder without an exec", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.dryRunOn = true

		oldClosure, gen, err := rollbackHint(surface, mach, guardPreset(t), "", testProfile)
		require.NoError(t, err)

		assert.Equal(t, dryRunOldClosure, oldClosure)
		assert.Equal(t, int64(2), gen)
		assert.Empty(t, surface.stepCalls())
	})

	t.Run("empty generation inventory fails closed", func(t *testing.T) {
		t.Parallel()

		bare := newGuardMachine(t, true, "auto")
		bare.MetaInspect.Store(&machine.MetaInspect{IsRoot: true, Architecture: "x86_64"})

		surface := newScriptedSurface()

		_, _, err := rollbackHint(surface, bare, guardPreset(t), "", testProfile)
		require.ErrorContains(t, err, "generation inventory is empty")
	})

	t.Run("unreadable generation link fails closed", func(t *testing.T) {
		t.Parallel()

		surface := newScriptedSurface()
		surface.script("resolve rollback target", fakeStepOutcome{err: exitErr(t, 1)})

		_, _, err := rollbackHint(surface, mach, guardPreset(t), "", testProfile)
		require.ErrorContains(t, err, "rollback-target hint")
	})
}

// writeStubGuardian materializes the stub guardian script the wire tests
// spawn through the local executioner. The stub logs every invocation,
// streams STATE records until the activated status, reads exactly one command
// frame, records it, and answers per PANIX_GUARD_STUB_MODE:
//
//   - precondition: STATE activating only, exit 4 (failed_precondition)
//   - cancel: STATE activating only, exit 6 (cancelled: inline converge)
//   - ack: CONFIRM_CONSUMED with the frame's rid, exit 0
//   - ack-revert: REVERT_REQUESTED with the frame's rid, exit 2
//   - mismatch: CONFIRM_CONSUMED with a mismatched rid, then it lingers so
//     the ack window expires while the exec is still live, exit 0
//   - none: no ack at all; the stub lingers so the ack window expires
//
// The verb branches (ctl/converge/inspect) serve the degraded and post-mortem
// paths: PANIX_GUARD_STUB_CTL_EXIT scenarizes the ctl exit, and
// PANIX_GUARD_STUB_VERDICT is the inspect verdict body.
func writeStubGuardian(t *testing.T) string {
	t.Helper()

	script := `#!/bin/sh
printf '%s\n' "$*" >> "$PANIX_GUARD_STUB_LOG"

case "$1" in
  ctl)
    # The degraded ctl fallback; the exit code is scenarizable (7 = the
    # delivered-but-unacked ack timeout, spec 9.5).
    exit "${PANIX_GUARD_STUB_CTL_EXIT:-0}"
    ;;
  converge)
    # The inline post-mortem converge: no mutation, exit converged.
    exit 0
    ;;
  inspect)
    # The one-shot verdict the outcome paths read back.
    printf '%s\n' "$PANIX_GUARD_STUB_VERDICT"
    exit 0
    ;;
esac

key="$PANIX_GUARD_STUB_KEY"
rec() {
  printf '@PG2 {"v":1,"k":"%s","w":"g","seq":%s,"ts":1700000000,"ev":"%s"%s}\n' "$key" "$1" "$2" "$3"
}

rec 1 STATE ',"st":"activating","pid":4242'

case "${PANIX_GUARD_STUB_MODE:-ack}" in
  precondition) sleep 0.3; exit 4 ;;
  cancel)       sleep 0.3; exit 6 ;;
esac

rec 2 STATE ',"st":"activated","pid":4242'

IFS= read -r frame
printf '%s\n' "$frame" >> "$PANIX_GUARD_STUB_FRAMES"
rid=$(printf '%s' "$frame" | tr -cd '0-9')

case "${PANIX_GUARD_STUB_MODE:-ack}" in
  ack)        rec 3 CONFIRM_CONSUMED ",\"rid\":$rid" ;;
  ack-revert) rec 3 REVERT_REQUESTED ",\"rid\":$rid" ;;
  mismatch)   rec 3 CONFIRM_CONSUMED ",\"rid\":999"
              sleep 7 ;;
  none)       sleep 7 ;;
esac

sleep 0.3
[ "${PANIX_GUARD_STUB_MODE:-ack}" = "ack-revert" ] && exit 2
exit 0
`

	path := filepath.Join(t.TempDir(), "panix-guard")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o700)) //nolint:gosec // test stub

	return path
}

// newWireTestMachine builds a root local machine with the magic tier, so the
// spawn argv runs the stub unwrapped and the confirm loop arms, with the
// remote check list the scenario needs.
func newWireTestMachine(t *testing.T, checks []string) *machine.Machine {
	t.Helper()

	mach := newGuardMachine(t, true, "magic")
	mach.HealthChecks = checks

	return mach
}

// newLocalGuardExecutioner builds a real (non-dry-run) local executioner for
// the wire-window tests: the spawn exec and the confirm loop run real
// processes through a PTY, exactly like the remote path's shapes.
func newLocalGuardExecutioner(t *testing.T, mach *machine.Machine) (*executioner.Executioner, *phaselogs.PhaseLog) {
	t.Helper()

	phaseLog := phaselogs.NewPhaseLog()
	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          t.Context(),
		Timeout:      2 * time.Minute,
		Xpath:        xpath.New("wire-test"),
		Machine:      mach,
		Phase:        phase.Activate,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	return exc, phaseLog
}

// stubDeployKey is the per-deploy key every stub-window test shares; the
// stub's records must carry it (nonce gating, spec 7).
const stubDeployKey = "stub-deploy-key"

// TestExecutionerSurfaceRunCapturesOutputOnFailure drives the PRODUCTION
// adapter (not the scripted double) against a real PTY exec that fails with
// output: the capture must deliver the output on failure and still propagate
// the error and its exit code. Regression: the OnSuccess-only capture never
// fired on failure, emptying the guardian's exit-1 inspect verdict and
// failing the legacy-sweep gate leg.
func TestExecutionerSurfaceRunCapturesOutputOnFailure(t *testing.T) {
	t.Parallel()

	exc, _ := newLocalGuardExecutioner(t, newGuardMachine(t, true, "auto"))
	surface := executionerSurface{ex: exc}

	out, err := surface.run("capture on failure", "running", "failed",
		[]string{"sh", "-c", `printf '{"status":"legacy","lock":false}'; exit 1`})
	require.Error(t, err)
	assert.JSONEq(t, `{"status":"legacy","lock":false}`, out)

	code, ok := exitCodeOfErr(err)
	require.True(t, ok, "the exec error must carry its exit code")
	assert.Equal(t, 1, code)

	out, err = surface.run("capture on success", "running", "failed",
		[]string{"sh", "-c", `printf '{"status":"ok"}'`})
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"ok"}`, out)
}

// newStubWindowPlan composes the guardedPlan for a stub-guardian window.
func newStubWindowPlan(t *testing.T, mach *machine.Machine, guardBin string) guardedPlan {
	t.Helper()

	preset := guardPreset(t)
	slotDir := t.TempDir()
	profile := preset.ProfilePath
	nixEnv := testNixEnv
	newClosure := "/nix/store/new"
	tier := preset.GuardTierValue()
	steps := composeStepLists(preset, "switch", profile, nixEnv, "/nix/store/old", newClosure)

	// The real slot carries the persisted guardian binary; the stub mirrors
	// that layout, so the inline converge and the inspect one-shots exec the
	// same script from the slot (guardianBinaryPath).
	stubBytes, err := os.ReadFile(guardBin)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(slotDir, guardianBinaryName), stubBytes, 0o700)) //nolint:gosec // test stub

	return guardedPlan{
		mach:              mach,
		preset:            preset,
		targetUser:        "",
		slotDir:           slotDir,
		guardBin:          guardBin,
		key:               stubDeployKey,
		gate:              GateMagic,
		activationTimeout: 30 * time.Second,
		confirmTimeout:    30 * time.Second,
		spawnArgv: SpawnArgv(guardBin, slotDir, stubDeployKey, profile, nixEnv, testNixStore,
			newClosure, 3, "switch", tier, GateMagic, 30*time.Second, 30*time.Second,
			false, mach.HealthChecksLocal, steps.Activation, steps.Commit, steps.Revert,
			steps.InvariantTarget, newClosure, "", true),
		profile: profile,
		nixEnv:  nixEnv,
	}
}

// hasCtlInvocation reports whether the stub log carries a ctl invocation
// (the fresh-connection fallback's fingerprint).
func hasCtlInvocation(stubLog string) bool {
	for line := range strings.SplitSeq(stubLog, "\n") {
		if strings.HasPrefix(line, "ctl ") {
			return true
		}
	}

	return false
}

// readStubFile reads one of the stub's capture files.
func readStubFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path) //nolint:gosec // repo-local test capture
	require.NoError(t, err)

	return string(content)
}

//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestGuardedWindow_WireConfirm(t *testing.T) {
	stub := writeStubGuardian(t)
	logPath := filepath.Join(t.TempDir(), "stub.log")
	framesPath := filepath.Join(t.TempDir(), "frames.txt")

	t.Setenv("PANIX_GUARD_STUB_LOG", logPath)
	t.Setenv("PANIX_GUARD_STUB_FRAMES", framesPath)
	t.Setenv("PANIX_GUARD_STUB_KEY", stubDeployKey)
	t.Setenv("PANIX_GUARD_STUB_MODE", "ack")

	mach := newWireTestMachine(t, nil)
	exc, phaseLog := newLocalGuardExecutioner(t, mach)

	err := runGuardedWindow(exc, newStubWindowPlan(t, mach, stub))
	require.NoError(t, err, "the wire-acked confirm commits the deploy")

	assert.JSONEq(t, "{\"c\":\"confirm\",\"rid\":1}\n", readStubFile(t, framesPath),
		"the confirm frame rides the stdin pipe with the per-deploy rid counter")

	invocations := readStubFile(t, logPath)
	assert.Contains(t, invocations, "start", "the stub guardian spawned once")
	assert.False(t, hasCtlInvocation(invocations), "the wire ack made the fresh-connection ctl fallback unnecessary")

	// Echo defense, composition level: the executioner disables the PTY echo
	// before the pump, so panix's own frame never re-enters the stream.
	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.NotContains(t, last.Output.String(), `{"c":"confirm"`, "an echoed frame must never re-enter the stream")
}

//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestGuardedWindow_WireAckFallbackToCtl(t *testing.T) {
	stub := writeStubGuardian(t)
	logPath := filepath.Join(t.TempDir(), "stub.log")
	framesPath := filepath.Join(t.TempDir(), "frames.txt")

	t.Setenv("PANIX_GUARD_STUB_LOG", logPath)
	t.Setenv("PANIX_GUARD_STUB_FRAMES", framesPath)
	t.Setenv("PANIX_GUARD_STUB_KEY", stubDeployKey)
	t.Setenv("PANIX_GUARD_STUB_MODE", "none")

	mach := newWireTestMachine(t, nil)
	exc, _ := newLocalGuardExecutioner(t, mach)

	err := runGuardedWindow(exc, newStubWindowPlan(t, mach, stub))
	require.NoError(t, err)

	assert.JSONEq(t, "{\"c\":\"confirm\",\"rid\":1}\n", readStubFile(t, framesPath))

	invocations := readStubFile(t, logPath)
	assert.True(t, hasCtlInvocation(invocations), "a silent wire falls back to the fresh-connection ctl")
	assert.Contains(t, invocations, "confirm", "the fallback carries the confirm verb")
}

//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestGuardedWindow_MismatchedRidNeverAcks(t *testing.T) {
	stub := writeStubGuardian(t)
	logPath := filepath.Join(t.TempDir(), "stub.log")
	framesPath := filepath.Join(t.TempDir(), "frames.txt")

	t.Setenv("PANIX_GUARD_STUB_LOG", logPath)
	t.Setenv("PANIX_GUARD_STUB_FRAMES", framesPath)
	t.Setenv("PANIX_GUARD_STUB_KEY", stubDeployKey)
	t.Setenv("PANIX_GUARD_STUB_MODE", "mismatch")

	mach := newWireTestMachine(t, nil)
	exc, _ := newLocalGuardExecutioner(t, mach)

	err := runGuardedWindow(exc, newStubWindowPlan(t, mach, stub))
	require.NoError(t, err)

	assert.True(t, hasCtlInvocation(readStubFile(t, logPath)),
		"a record with a mismatched rid is never an ack; the ctl fallback answers")
}

//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestGuardedWindow_CheckFailureRequestsRevertOverWire(t *testing.T) {
	stub := writeStubGuardian(t)
	logPath := filepath.Join(t.TempDir(), "stub.log")
	framesPath := filepath.Join(t.TempDir(), "frames.txt")

	t.Setenv("PANIX_GUARD_STUB_LOG", logPath)
	t.Setenv("PANIX_GUARD_STUB_FRAMES", framesPath)
	t.Setenv("PANIX_GUARD_STUB_KEY", stubDeployKey)
	t.Setenv("PANIX_GUARD_STUB_MODE", "ack-revert")

	mach := newWireTestMachine(t, []string{"false"})
	exc, _ := newLocalGuardExecutioner(t, mach)

	err := runGuardedWindow(exc, newStubWindowPlan(t, mach, stub))
	require.Error(t, err, "the stub reverted (exit 2)")

	assert.Contains(t, err.Error(), "reverted")
	assert.JSONEq(t, "{\"c\":\"revert\",\"rid\":1}\n", readStubFile(t, framesPath),
		"the first failing check requests the revert over the wire")
	assert.False(t, hasCtlInvocation(readStubFile(t, logPath)), "the revert ack rode the wire")
}

// TestInlineDecideOutcome_UsesRecordContext pins the TXN-context reporting:
// the post-mortem convergence composes its decide argv from the records'
// old/new/gen (the streamed/log context), never from a panix-side capture.
// commandOutputs collects every registered command log's output text.
func commandOutputs(t *testing.T, phaseLog *phaselogs.PhaseLog) []string {
	t.Helper()

	var out []string

	for i := 0; ; i++ {
		log, ok := phaseLog.CommandLogs.Get(i)
		if !ok {
			break
		}

		out = append(out, log.Output.String())
	}

	return out
}

// TestGuardedWindow_ExitMatrix pins the exit-code consumption through the
// shared per-verb constants (spec 9.5): failed_precondition reports zero
// effects, and cancelled (6) runs the inline converge and reads the converged
// outcome back through an inspect one-shot.
func TestGuardedWindow_ExitMatrix(t *testing.T) {
	t.Run("failed precondition reports zero effects", func(t *testing.T) {
		stub := writeStubGuardian(t)
		logPath := filepath.Join(t.TempDir(), "stub.log")
		framesPath := filepath.Join(t.TempDir(), "frames.txt")

		t.Setenv("PANIX_GUARD_STUB_LOG", logPath)
		t.Setenv("PANIX_GUARD_STUB_FRAMES", framesPath)
		t.Setenv("PANIX_GUARD_STUB_KEY", stubDeployKey)
		t.Setenv("PANIX_GUARD_STUB_MODE", "precondition")

		mach := newWireTestMachine(t, nil)
		exc, _ := newLocalGuardExecutioner(t, mach)

		err := runGuardedWindow(exc, newStubWindowPlan(t, mach, stub))
		require.ErrorContains(t, err, "precondition check failed, zero effects")
		assert.NotContains(t, readStubFile(t, logPath), "converge", "exit 4 never converges")
	})

	//nolint:paralleltest // t.Setenv forbids t.Parallel
	t.Run("cancelled runs the inline converge", func(t *testing.T) {
		stub := writeStubGuardian(t)
		logPath := filepath.Join(t.TempDir(), "stub.log")
		framesPath := filepath.Join(t.TempDir(), "frames.txt")

		t.Setenv("PANIX_GUARD_STUB_LOG", logPath)
		t.Setenv("PANIX_GUARD_STUB_FRAMES", framesPath)
		t.Setenv("PANIX_GUARD_STUB_KEY", stubDeployKey)
		t.Setenv("PANIX_GUARD_STUB_MODE", "cancel")
		t.Setenv("PANIX_GUARD_STUB_VERDICT", `{"status":"committed","terminal":true,"rc":0,"lock":false}`)

		mach := newWireTestMachine(t, nil)
		exc, _ := newLocalGuardExecutioner(t, mach)

		err := runGuardedWindow(exc, newStubWindowPlan(t, mach, stub))
		require.NoError(t, err, "the converge landed on committed")

		invocations := readStubFile(t, logPath)
		assert.True(t, hasCtlInvocation(invocations) || strings.Contains(invocations, "\nconverge "),
			"the inline converge must run after a cancelled window")
		assert.Contains(t, invocations, "\ninspect ", "the converged outcome is read back through inspect")
	})
}

// TestGuardedWindow_CtlAckTimeoutNote pins the ack-timeout semantics (spec
// 9.5 ctl exit 7): the request was delivered but no ack arrived within the
// wait, so it may still take effect, and the caveat lands in the narrative.
func TestGuardedWindow_CtlAckTimeoutNote(t *testing.T) {
	stub := writeStubGuardian(t)
	logPath := filepath.Join(t.TempDir(), "stub.log")
	framesPath := filepath.Join(t.TempDir(), "frames.txt")

	t.Setenv("PANIX_GUARD_STUB_LOG", logPath)
	t.Setenv("PANIX_GUARD_STUB_FRAMES", framesPath)
	t.Setenv("PANIX_GUARD_STUB_KEY", stubDeployKey)
	t.Setenv("PANIX_GUARD_STUB_MODE", "none")
	t.Setenv("PANIX_GUARD_STUB_CTL_EXIT", "7")

	mach := newWireTestMachine(t, nil)
	exc, phaseLog := newLocalGuardExecutioner(t, mach)

	err := runGuardedWindow(exc, newStubWindowPlan(t, mach, stub))
	require.NoError(t, err)

	assert.True(t, hasCtlInvocation(readStubFile(t, logPath)), "the fallback ctl ran")
	assert.Contains(t, strings.Join(commandOutputs(t, phaseLog), "\n"),
		"may still take effect", "the ack-timeout caveat is its own outcome path")
}

// runInlineDecide scripts the post-mortem converge outcome and the read-back
// inspect verdict, then runs the inline post-mortem against a scripted
// surface. An empty verdict scripts no read-back (the converge failed).
func runInlineDecide(t *testing.T, startErr, convergeErr error, verdictJSON string) (*scriptedSurface, error) {
	t.Helper()

	surface := newScriptedSurface()
	surface.script("guard post-mortem convergence", fakeStepOutcome{err: convergeErr})

	switch {
	case verdictJSON != "":
		surface.script("guard slot inspect", fakeStepOutcome{output: verdictJSON + "\n"})
	case convergeErr == nil:
		// A converged run with no scripted verdict reads back an unparseable
		// one: the unknown-outcome path without a window error.
		surface.script("guard slot inspect", fakeStepOutcome{err: exitErr(t, guard.InspectExitUnparseable)})
	}

	err := inlineDecideOutcome(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
		"/run/panix-guard/v2/slot", testProfile, testNixEnv, time.Minute, startErr)

	return surface, err
}

// TestInlineDecideOutcome_Converge pins the post-mortem convergence (spec
// 9.3, 9.4): panix execs converge WITHOUT --truncate (truncation is the
// pre-start caller's job; the post-mortem keeps the log for reporting, and a
// truncated log would blind the read-back) and passes NO step lists or
// recovered identity beyond the profile/nix-env context fallback (converge is
// self-sufficient via the TXN record), maps the exit
// codes, and reads the converged outcome back through an inspect one-shot.
func TestInlineDecideOutcome_Converge(t *testing.T) {
	t.Parallel()

	converged := `{"status":"committed","terminal":true,"old":"/nix/store/record-old","new":"/nix/store/record-new","gen":9,"rc":0,"key":"prior-key","lock":false}`
	status := func(s string) string {
		return strings.Replace(converged, `"status":"committed"`, `"status":"`+s+`"`, 1)
	}

	t.Run("converged to committed", func(t *testing.T) {
		t.Parallel()

		surface, err := runInlineDecide(t, errors.New("guardian died"), nil, converged)
		require.NoError(t, err, "the deploy actually completed before the guardian died")

		convergeCall := surface.stepCalls()[0]
		assert.Contains(t, convergeCall, "converge --dir /run/panix-guard/v2/slot")
		assert.Contains(t, convergeCall, "--profile "+testProfile, "the profile fallback rides the post-mortem argv")
		assert.Contains(t, convergeCall, "--nix-env "+testNixEnv, "the resolved nix-env rides the post-mortem argv")
		assert.NotContains(t, convergeCall, "--truncate", "the post-mortem keeps the log for reporting (spec 9.3)")
		assert.NotContains(t, convergeCall, "--commit-argv", "converge needs no argv-supplied step lists")
		assert.NotContains(t, convergeCall, "--old", "converge needs no recovered context")
		assert.NotContains(t, convergeCall, "--key", "converge self-locks and recovers the key from the records")
	})

	t.Run("converged to reverted", func(t *testing.T) {
		t.Parallel()

		_, err := runInlineDecide(t, errors.New("guardian died"), nil, status("reverted"))
		require.ErrorContains(t, err, "reverted to the previous generation")
	})

	t.Run("converged to failed_precondition reports zero effects", func(t *testing.T) {
		t.Parallel()

		_, err := runInlineDecide(t, errors.New("guardian died"), nil, status("failed_precondition"))
		require.ErrorContains(t, err, "precondition check failed, zero effects")
	})

	t.Run("convergence failed reports honestly", func(t *testing.T) {
		t.Parallel()

		surface, err := runInlineDecide(t, errors.New("guardian died"), exitErr(t, guard.ConvergeExitFailed), "")
		require.ErrorContains(t, err, "post-mortem convergence failed")
		assert.Len(t, surface.stepCalls(), 1, "a failed converge reads back no verdict")
	})

	t.Run("locked slot reports honestly", func(t *testing.T) {
		t.Parallel()

		_, err := runInlineDecide(t, errors.New("guardian died"), exitErr(t, guard.ConvergeExitLocked), "")
		require.ErrorContains(t, err, "still locked by a live transaction")
	})

	t.Run("unreadable read-back never reports success without a window error", func(t *testing.T) {
		t.Parallel()

		// The post-disconnect poll path calls with a nil window error: a nil
		// wrap must not turn the unknown outcome into a success report.
		_, err := runInlineDecide(t, nil, nil, "")
		require.ErrorContains(t, err, "converged state unreadable")
	})
}

// scriptedVerdict is one inspect verdict outcome for the scripted surface.
func scriptedVerdict(verdictJSON string) fakeStepOutcome {
	return fakeStepOutcome{output: verdictJSON + "\n"}
}

// runAwaitTerminal scripts the inspect poll queue (and a succeeding inline
// converge), then runs the post-disconnect resolution against a scripted
// surface.
func runAwaitTerminal(t *testing.T, startedAt time.Time, bound time.Duration, inspect ...fakeStepOutcome) (*scriptedSurface, error) {
	t.Helper()

	surface := newScriptedSurface()
	surface.script("guard slot inspect", inspect...)
	surface.script("guard post-mortem convergence", fakeStepOutcome{})

	err := awaitTerminalOutcome(surface, newGuardMachine(t, true, "auto"), guardPreset(t), "",
		"/run/panix-guard/v2/slot", testProfile, testNixEnv, startedAt, bound)

	return surface, err
}

// TestAwaitTerminalOutcome_InspectPolls pins the post-disconnect resolution
// (spec 9.4): short inspect one-shot polls (master connection, small timeout)
// until a terminal verdict, then the outcome maps from the verdict; a free
// lock with a non-terminal state is a dead guardian and hands over to the
// inline converge immediately; a still-non-terminal state at the deadline
// converges inline and reads back through another inspect.
func TestAwaitTerminalOutcome_InspectPolls(t *testing.T) {
	t.Parallel()

	activating := `{"status":"activating","terminal":false,"lock":true}`
	reverted := `{"status":"reverted","terminal":true,"err":"systemd units failed","rc":2,"lock":false}`

	t.Run("terminal verdict resolves the outcome", func(t *testing.T) {
		t.Parallel()

		_, err := runAwaitTerminal(t, time.Now().Add(-time.Minute), 5*time.Minute,
			scriptedVerdict(activating), scriptedVerdict(reverted))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "systemd units failed", "the verdict's err excerpt surfaces")
	})

	t.Run("terminal activation_exited is a success", func(t *testing.T) {
		t.Parallel()

		_, err := runAwaitTerminal(t, time.Now(), time.Minute,
			scriptedVerdict(`{"status":"activation_exited","terminal":true,"rc":0,"lock":false}`))
		require.NoError(t, err, "minimal tier: the activation ran and exited, no commit or revert applies")
	})

	t.Run("terminal failed_precondition reports zero effects", func(t *testing.T) {
		t.Parallel()

		_, err := runAwaitTerminal(t, time.Now(), time.Minute,
			scriptedVerdict(`{"status":"failed_precondition","terminal":true,"rc":0,"lock":false}`))
		require.ErrorContains(t, err, "precondition check failed, zero effects")
	})

	t.Run("a free lock with a non-terminal state converges immediately", func(t *testing.T) {
		t.Parallel()

		// The deadline is far ahead: the free lock (a dead guardian) must not
		// wait for it - the poll stops at the first verdict and converges.
		started := time.Now()

		_, err := runAwaitTerminal(t, started, time.Hour,
			scriptedVerdict(`{"status":"activating","terminal":false,"lock":false}`),
			scriptedVerdict(`{"status":"reverted","terminal":true,"rc":0,"lock":false}`))
		require.ErrorContains(t, err, "reverted to the previous generation",
			"the converged outcome is the deploy's own, not an unknown")

		assert.Less(t, time.Since(started), 5*time.Second, "the dead guardian must not be polled to the deadline")
	})

	t.Run("non-terminal state at the deadline converges inline", func(t *testing.T) {
		t.Parallel()

		// The deadline is already past: no poll rounds, straight to the inline
		// converge, then the follow-up inspect reads the converged verdict.
		surface, err := runAwaitTerminal(t, time.Now().Add(-time.Minute), 0,
			scriptedVerdict(`{"status":"committed","terminal":true,"rc":0,"lock":false}`))
		require.NoError(t, err, "the converge landed on committed")

		convergeCall := strings.Join(surface.stepCalls(), "\n")
		assert.Contains(t, convergeCall, "converge --dir /run/panix-guard/v2/slot")
		assert.NotContains(t, convergeCall, "--truncate", "the post-mortem keeps the log for reporting (spec 9.3)")
	})
}

// TestAwaitTerminalOutcome_VanishedSlot pins the mid-window tmpfs-clear handover
// (spec 9.3 step 6): a missing slot or log (inspect exit 4) reports the honest
// unknown and leaves the resolution to the next deploy's pre-start sweep.
func TestAwaitTerminalOutcome_VanishedSlot(t *testing.T) {
	t.Parallel()

	_, err := runAwaitTerminal(t, time.Now(), time.Minute,
		fakeStepOutcome{output: "absent"})
	require.ErrorContains(t, err, "guard slot vanished mid-window")
}

// TestCancellationOutcome_GiveUpCancel pins the give-up path's cancel send
// (spec 8): before reporting, the outcome composes exactly ONE
// fresh-connection `ctl revert-request --cause cancel` whose delivery failure
// is swallowed (best effort; the guardian converges autonomously either way),
// and the returned error stays cancel-preserving for the workflow. The
// cancel-vs-failed distinction lives in this outcome's report by design: the
// record cannot carry a cause over payload-less signals (spec 8 design note).
func TestCancellationOutcome_GiveUpCancel(t *testing.T) {
	t.Parallel()

	surface := newScriptedSurface()
	// The ctl delivery failure is swallowed (best effort): the machine may
	// be unreachable at give-up time.
	surface.script("guard ctl revert-request", fakeStepOutcome{err: exitErr(t, 255)})
	surface.script("guard slot inspect", fakeStepOutcome{
		output: `{"status":"activated","terminal":false,"lock":true}` + "\n",
	})

	err := cancellationOutcome(surface, newGuardMachine(t, true, "magic"), guardPreset(t), "",
		"/run/panix-guard/v2/slot", "cancel-deploy-key", context.Canceled, context.Canceled)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled, "the outcome stays cancel-preserving for the workflow")
	assert.Contains(t, err.Error(), "activation cancelled")

	calls := surface.stepCalls()

	ctlCalls := 0

	for _, call := range calls {
		if !strings.Contains(call, "guard ctl revert-request") {
			continue
		}

		ctlCalls++

		assert.Contains(t, call, "--cause cancel", "the give-up cancel rides the --cause narration (spec 8)")
		assert.Contains(t, call, "--key cancel-deploy-key")
		assert.Contains(t, call, "revert-request")
	}

	assert.Equal(t, 1, ctlCalls, "the give-up path sends exactly one best-effort cancel")

	ctlIdx, inspectIdx := callIndex(calls, "guard ctl revert-request"), callIndex(calls, "guard slot inspect")
	assert.GreaterOrEqual(t, ctlIdx, 0, "the cancel must be delivered at all")
	assert.Less(t, ctlIdx, inspectIdx, "the cancel precedes the reporting inspect one-shot")
}

// TestCtlArgvCause pins the cause composition: the flag-less shape stays
// byte-identical for the existing call sites, and a non-empty cause inserts
// --cause before the verb without touching the exit-code contract.
func TestCtlArgvCause(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]string{"g", "ctl", "--dir", "/slot", "--key", "k", "--wait", "5s", "confirm"},
		CtlArgv("g", "/slot", "k", "confirm", 5*time.Second, ""))
	assert.Equal(t,
		[]string{"g", "ctl", "--dir", "/slot", "--key", "k", "--wait", "5s", "--cause", "cancel", "revert-request"},
		CtlArgv("g", "/slot", "k", "revert-request", 5*time.Second, GuardCancelCause))
}
