package fleet_test

import (
	"fmt"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
)

var testWorkflowPhases = []phase.Phase{phase.Inspect, phase.Build, phase.Activate}

func buildTestFleet(machines ...*machine.Machine) *fleet.Fleet {
	machinesMap := atomicorderedmap.New[string, *machine.Machine]()
	for i, m := range machines {
		machinesMap.Set(fmt.Sprintf("m%d", i), m)
	}

	cfg := &installable.Installable{}
	cfg.Logs = logs.New()
	cfg.Machines = machinesMap

	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Installables = atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]]()

	attrMap := atomicorderedmap.New[string, *installable.Installable]()
	attrMap.Set("cfg0", cfg)
	flakeObj.Installables.Set("nixosConfigurations", attrMap)

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()
	flakesMap.Set("flake0", flakeObj)

	return &fleet.Fleet{Flakes: flakesMap}
}

func newMachine(ph phase.Phase, status stats.StatsState, xp string) *machine.Machine {
	mach := &machine.Machine{}
	mach.Xpath = xpath.New(xp)
	mach.State = atomicpointer.New[machine.State]()
	mach.State.Store(&machine.State{
		Phase:  ph,
		Status: status,
	})

	return mach
}

func TestRecalculatePhaseStatus_SkipsMachinesWithEmptyPhase(t *testing.T) {
	t.Parallel()

	// Machine that hasn't started — Phase and Status are empty strings.
	notStarted := newMachine("", "", "flake0/cfg0/m0")
	fleetInst := buildTestFleet(notStarted)

	spp := fleetInst.RecalculatePhaseStatus(testWorkflowPhases)

	// The "" key must NOT exist in the map — it would shift spp.Last()
	// away from the real last workflow phase and break the PhaseFlow DONE column.
	assert.False(t, spp.Exists(phase.Phase("")),
		"empty-phase key must not be present in the stats map")

	// The map should contain exactly the workflow phases.
	assert.Equal(t, len(testWorkflowPhases), spp.Len(),
		"stats map should contain exactly the workflow phases")
}

func TestRecalculatePhaseStatus_DoneInLastPhase(t *testing.T) {
	t.Parallel()

	doneMachine := newMachine(phase.Activate, stats.Done, "flake0/cfg0/m0")
	fleetInst := buildTestFleet(doneMachine)

	spp := fleetInst.RecalculatePhaseStatus(testWorkflowPhases)

	lastPhase := testWorkflowPhases[len(testWorkflowPhases)-1]
	statsPack, ok := spp.Get(lastPhase)
	assert.True(t, ok, "last workflow phase should exist in stats map")
	assert.Len(t, statsPack[stats.Done], 1,
		"last phase should have 1 Done machine")
}

func TestRecalculatePhaseStatus_MixedStates(t *testing.T) {
	t.Parallel()

	notStarted := newMachine("", "", "flake0/cfg0/m0")
	running := newMachine(phase.Build, stats.Running, "flake0/cfg0/m1")
	done := newMachine(phase.Activate, stats.Done, "flake0/cfg0/m2")
	failed := newMachine(phase.Inspect, stats.Failed, "flake0/cfg0/m3")
	fleetInst := buildTestFleet(notStarted, running, done, failed)

	spp := fleetInst.RecalculatePhaseStatus(testWorkflowPhases)

	// No empty-phase key.
	assert.False(t, spp.Exists(phase.Phase("")),
		"empty-phase key must not be present")

	// Build phase should have 1 Running.
	buildStats, ok := spp.Get(phase.Build)
	assert.True(t, ok)
	assert.Len(t, buildStats[stats.Running], 1, "Build should have 1 Running")

	// Activate (last) phase should have 1 Done.
	activateStats, ok := spp.Get(phase.Activate)
	assert.True(t, ok)
	assert.Len(t, activateStats[stats.Done], 1, "Activate should have 1 Done")

	// Inspect phase should have 1 Failed.
	inspectStats, ok := spp.Get(phase.Inspect)
	assert.True(t, ok)
	assert.Len(t, inspectStats[stats.Failed], 1, "Inspect should have 1 Failed")
}

// TestRecalculatePhaseStatus_StaleEmptyKeyPersistsAcrossResets verifies that
// the PhaseFlow fix (using spp.Get instead of spp.Last) is necessary: even
// with the guard in RecalculatePhaseStatus, a stale "" key from a previous
// cycle persists across Reset() calls because Reset only clears values, not keys.
func TestRecalculatePhaseStatus_StaleEmptyKeyPersistsAcrossResets(t *testing.T) {
	t.Parallel()

	notStarted := newMachine("", "", "flake0/cfg0/m0")
	fleetInst := buildTestFleet(notStarted)

	// First call with the guard — no "" key should be added.
	spp := fleetInst.RecalculatePhaseStatus(testWorkflowPhases)
	assert.False(t, spp.Exists(phase.Phase("")),
		"no empty-phase key after first recalculation")

	// Manually simulate pre-fix pollution: add a "" key.
	spp.DeepSet(phase.Phase(""), "", xpath.New("flake0/cfg0/m0"))
	assert.True(t, spp.Exists(phase.Phase("")),
		"manually added empty-phase key should exist")

	// Second call — Reset() preserves keys, so the "" key persists.
	// But with the guard, no new entries are added to it.
	spp = fleetInst.RecalculatePhaseStatus(testWorkflowPhases)
	assert.True(t, spp.Exists(phase.Phase("")),
		"stale empty-phase key persists across Reset (expected — PhaseFlow fix handles this)")

	// The "" key's StatsPack should have no Done entries — the guarded loop
	// skips machines with empty Phase/Status.
	emptyStats, ok := spp.Get(phase.Phase(""))
	assert.True(t, ok)
	assert.Empty(t, emptyStats[stats.Done],
		"stale empty-phase key should have no Done entries")
}
