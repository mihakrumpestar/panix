package activate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/stringbyte"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHookTestMachine builds a LOCAL machine with inspect data, so activation
// hooks and scripts run for real through the local transport.
func newHookTestMachine(t *testing.T) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.Name = stringbyte.StringByte("my-machine")
	mach.MetaInspect.Store(&machine.MetaInspect{
		IsRoot:      true,
		Generations: &machine.Generations{Current: 42, Available: []uint{41, 42}},
	})

	return mach
}

// newHookTestExecutioner builds a real (non dry-run) local executioner: hooks
// and activation scripts execute, and the phase log records every step.
func newHookTestExecutioner(t *testing.T, mach *machine.Machine) (*executioner.Executioner, *phaselogs.PhaseLog) {
	t.Helper()

	phaseLog := phaselogs.NewPhaseLog()
	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		Timeout:      10 * time.Second,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Activate,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	return exc, phaseLog
}

// newHookTestFleetLeaf assembles a user-level installable whose activation
// script lives under the closure dir passed to executeActivation, so no
// sudo, profile or nix state is touched.
func newHookTestFleetLeaf(t *testing.T, profilePath string) *fleet.FleetLeaf {
	t.Helper()

	inst := &installable.Installable{
		Preset: installable.Preset{
			ProfilePath:           profilePath,
			ActivationPath:        "bin/activate",
			ActivationModes:       []string{"switch", "dry-activate"},
			NonMutatingModes:      []string{"dry-activate"},
			ActivationDefaultMode: "switch",
			IsSystemLevel:         new(false),
		},
		Name:     "my-installable",
		Machines: atomicorderedmap.New[string, *machine.Machine](),
	}

	return &fleet.FleetLeaf{Installable: inst, Machine: newHookTestMachine(t)}
}

// writeActivateScript materializes closureDir/bin/activate writing a marker
// file before exiting with the given code.
func writeActivateScript(t *testing.T, closureDir string, exitCode int) {
	t.Helper()

	binDir := filepath.Join(closureDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o750))

	script := fmt.Sprintf("#!/bin/sh\ntouch '%s'\nexit %d\n", filepath.Join(closureDir, "activated"), exitCode)
	// #nosec G306 -- the exec bit is required: the activation script must be runnable
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "activate"), []byte(script), 0o700))
}

// envDumpHook returns a hook command writing the PANIX_* hook environment to
// path, pipe-separated in the fixed envHooks order.
var envHookNames = []string{
	"PANIX_HOOK_PHASE",
	"PANIX_CLOSURE",
	"PANIX_PROFILE_PATH",
	"PANIX_PREV_GENERATION",
	"PANIX_ACTIVATION_MODE",
	"PANIX_INSTALLABLE",
	"PANIX_MACHINE",
}

func envDumpHook(path string) attributes.HookCommand {
	refs := make([]string, 0, len(envHookNames))
	for _, name := range envHookNames {
		refs = append(refs, `"$`+name+`"`)
	}

	return attributes.HookCommand(fmt.Sprintf("printf '%%s\\n' %s > '%s'", joinWords(refs), path))
}

// joinWords joins already-quoted shell words with single spaces.
func joinWords(words []string) string {
	return strings.Join(words, " ")
}

// envDumpExpectation renders the expected envDumpHook file content for the
// given fleet leaf, phase and mode.
func envDumpExpectation(leaf *fleet.FleetLeaf, hookPhase, mode, closure string) string {
	mi := leaf.Machine.MetaInspect.Load()

	prevGeneration := ""
	if mi != nil && mi.Generations != nil {
		prevGeneration = strconv.FormatUint(uint64(mi.Generations.Current), 10)
	}

	values := []string{
		hookPhase,
		closure,
		leaf.Installable.Preset.ProfilePath,
		prevGeneration,
		mode,
		leaf.Installable.Name.String(),
		leaf.Machine.Name.String(),
	}

	return strings.Join(values, "\n") + "\n"
}

func readMarker(t *testing.T, path string) string {
	t.Helper()

	// #nosec G304 -- the path is a test-owned t.TempDir() path, not user input
	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

func commandDescriptions(t *testing.T, phaseLog *phaselogs.PhaseLog) []string {
	t.Helper()

	var out []string

	for i := 0; ; i++ {
		logEntry, ok := phaseLog.CommandLogs.Get(i)
		if !ok {
			break
		}

		out = append(out, logEntry.Description)
	}

	return out
}

// Pre and post hooks run around a successful activation, each receiving the
// full PANIX_* environment; the phase field distinguishes them.
func TestExecuteActivation_HooksRunWithEnv(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	closure := filepath.Join(base, "closure")
	writeActivateScript(t, closure, 0)

	leaf := newHookTestFleetLeaf(t, filepath.Join(base, "profile"))
	mach := leaf.Machine

	leaf.Machine.ActivationHooks = attributes.ActivationHooks{
		Pre:  []attributes.HookCommand{envDumpHook(filepath.Join(base, "pre-env"))},
		Post: []attributes.HookCommand{envDumpHook(filepath.Join(base, "post-env"))},
	}

	exc, phaseLog := newHookTestExecutioner(t, mach)

	err := executeActivation(exc, flags.ActivationMode{}, nixver.FlavorNix, leaf, closure, &leaf.Installable.Nix)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(closure, "activated"), "activation must have run")
	assert.Equal(t, envDumpExpectation(leaf, "pre", "switch", closure), readMarker(t, filepath.Join(base, "pre-env")))
	assert.Equal(t, envDumpExpectation(leaf, "post", "switch", closure), readMarker(t, filepath.Join(base, "post-env")))

	descriptions := commandDescriptions(t, phaseLog)
	assert.Equal(t, []string{"pre activation hook 1", "activate", "post activation hook 1"}, descriptions,
		"hooks must wrap the activation in order")
}

// A failing pre hook fails the machine before activation: the activation
// never runs and the system stays untouched (no rollback either).
func TestExecuteActivation_PreHookFailurePreventsActivation(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	closure := filepath.Join(base, "closure")
	writeActivateScript(t, closure, 0)

	leaf := newHookTestFleetLeaf(t, filepath.Join(base, "profile"))
	leaf.Machine.AutoRollback = true
	leaf.Machine.ActivationHooks = attributes.ActivationHooks{
		Pre: []attributes.HookCommand{"exit 1"},
	}

	exc, _ := newHookTestExecutioner(t, leaf.Machine)

	err := executeActivation(exc, flags.ActivationMode{}, nixver.FlavorNix, leaf, closure, &leaf.Installable.Nix)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pre activation hooks failed")

	assert.NoFileExists(t, filepath.Join(closure, "activated"), "activation must be skipped")
}

// Post hooks are skipped for non-mutating modes: dry-activate changed
// nothing, so there is nothing to follow up.
func TestExecuteActivation_PostHooksSkippedForNonMutatingMode(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	closure := filepath.Join(base, "closure")
	writeActivateScript(t, closure, 0)

	leaf := newHookTestFleetLeaf(t, filepath.Join(base, "profile"))
	leaf.Machine.ActivationHooks = attributes.ActivationHooks{
		Post: []attributes.HookCommand{"exit 1"},
	}

	exc, phaseLog := newHookTestExecutioner(t, leaf.Machine)

	// The dry-activate override exercises the CLI override resolution path.
	err := executeActivation(exc, flags.ActivationMode{AllTypes: "dry-activate"}, nixver.FlavorNix, leaf, closure, &leaf.Installable.Nix)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(closure, "activated"), "activation must have run")
	descriptions := commandDescriptions(t, phaseLog)
	assert.NotContains(t, descriptions, "post activation hook 1", "post hooks must be skipped for dry-activate")
}

// A failing post hook fails the phase without rolling back the successful
// activation: the rollback path belongs to activation failures only.
func TestExecuteActivation_PostHookFailureFailsWithoutRollback(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	closure := filepath.Join(base, "closure")
	writeActivateScript(t, closure, 0)

	leaf := newHookTestFleetLeaf(t, filepath.Join(base, "profile"))
	leaf.Machine.AutoRollback = true
	leaf.Machine.ActivationHooks = attributes.ActivationHooks{
		Post: []attributes.HookCommand{"exit 1"},
	}

	exc, phaseLog := newHookTestExecutioner(t, leaf.Machine)

	err := executeActivation(exc, flags.ActivationMode{}, nixver.FlavorNix, leaf, closure, &leaf.Installable.Nix)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post activation hooks failed")

	assert.FileExists(t, filepath.Join(closure, "activated"), "activation must have run before the post hook")

	for _, description := range commandDescriptions(t, phaseLog) {
		assert.NotContains(t, description, "find generation closure")
		assert.NotContains(t, description, "auto rollback")
	}
}

// Hooks belong to executeActivation only: during the auto-rollback the pre
// hooks do not run a second time and the post hooks never run, even though
// the rollback activation itself succeeds.
func TestExecuteActivation_HooksNotTriggeredDuringRollback(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	failingClosure := filepath.Join(base, "failing-closure")
	writeActivateScript(t, failingClosure, 1)

	rollbackClosure := filepath.Join(base, "rollback-closure")
	writeActivateScript(t, rollbackClosure, 0)

	profilePath := filepath.Join(base, "profile")
	require.NoError(t, os.Symlink(rollbackClosure, profilePath+"-42-link"))

	leaf := newHookTestFleetLeaf(t, profilePath)
	leaf.Machine.AutoRollback = true

	preRuns := filepath.Join(base, "pre-runs")
	leaf.Machine.ActivationHooks = attributes.ActivationHooks{
		Pre:  []attributes.HookCommand{attributes.HookCommand(fmt.Sprintf("echo pre >> '%s'", preRuns))},
		Post: []attributes.HookCommand{attributes.HookCommand(fmt.Sprintf("touch '%s'", filepath.Join(base, "post-ran")))},
	}

	exc, phaseLog := newHookTestExecutioner(t, leaf.Machine)

	err := executeActivation(exc, flags.ActivationMode{}, nixver.FlavorNix, leaf, failingClosure, &leaf.Installable.Nix)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto-rollback succeeded", "the rollback activation must succeed")

	assert.Equal(t, "pre\n", readMarker(t, preRuns), "pre hooks must run exactly once, not during rollback")
	assert.NoFileExists(t, filepath.Join(base, "post-ran"), "post hooks must not run after a failed activation")
	assert.Contains(t, commandDescriptions(t, phaseLog), "find generation closure", "rollback must have run")
}

// Dry-run logs the hook steps around the activation without executing
// anything: the hooks write marker files a real run would leave behind.
func TestExecuteActivation_DryRunLogsHooksWithoutExecuting(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	closure := filepath.Join(base, "closure")
	writeActivateScript(t, closure, 0)

	leaf := newHookTestFleetLeaf(t, filepath.Join(base, "profile"))
	leaf.Machine.ActivationHooks = attributes.ActivationHooks{
		Pre:  []attributes.HookCommand{attributes.HookCommand(fmt.Sprintf("touch '%s'", filepath.Join(base, "pre-ran")))},
		Post: []attributes.HookCommand{attributes.HookCommand(fmt.Sprintf("touch '%s'", filepath.Join(base, "post-ran")))},
	}

	_, phaseLog := newHookTestExecutioner(t, leaf.Machine)

	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		DryRun:       true,
		Xpath:        xpath.New("test"),
		Machine:      leaf.Machine,
		Phase:        phase.Activate,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	err := executeActivation(exc, flags.ActivationMode{}, nixver.FlavorNix, leaf, closure, &leaf.Installable.Nix)
	require.NoError(t, err)

	assert.Equal(t, []string{"pre activation hook 1", "activate", "post activation hook 1"}, commandDescriptions(t, phaseLog))
	assert.NoFileExists(t, filepath.Join(base, "pre-ran"), "dry-run must not execute the pre hook")
	assert.NoFileExists(t, filepath.Join(base, "post-ran"), "dry-run must not execute the post hook")
	assert.NoFileExists(t, filepath.Join(closure, "activated"), "dry-run must not execute the activation")
}
