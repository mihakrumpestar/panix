package phaseflow

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
)

func TestRender_DoneColumnFromLastPhase(t *testing.T) {
	t.Parallel()

	workflowPhases := []phase.Phase{phase.Inspect, phase.Build, phase.Activate}

	// Build a StatisticsPerPhase where the last phase has 2 Done machines
	// and Build has 1 Running.
	spp := stats.New(workflowPhases)
	spp.DeepSet(phase.Activate, stats.Done, xpath.New("flake0/cfg0/m0"))
	spp.DeepSet(phase.Activate, stats.Done, xpath.New("flake0/cfg0/m1"))
	spp.DeepSet(phase.Build, stats.Running, xpath.New("flake0/cfg0/m2"))

	fleetInst := &fleet.Fleet{CacheStatisticsPerPhase: spp}
	phaseFlow := New(fleetInst, colorscheme.DefaultColorScheme(), workflowPhases)

	phaseFlow.Render(200)

	// p.data should have len(workflowPhases)+1 entries: [Inspect, Build, Activate, DONE]
	assert.Len(t, phaseFlow.data, len(workflowPhases)+1,
		"data should have one entry per phase + DONE")

	// Inspect: 0 running, 0 failed
	assert.Equal(t, 0, phaseFlow.data[0].Running)
	assert.Equal(t, 0, phaseFlow.data[0].Failed)

	// Build: 1 running, 0 failed
	assert.Equal(t, 1, phaseFlow.data[1].Running)
	assert.Equal(t, 0, phaseFlow.data[1].Failed)

	// Activate: 0 running, 0 failed (Done is read separately for the DONE column)
	assert.Equal(t, 0, phaseFlow.data[2].Running)
	assert.Equal(t, 0, phaseFlow.data[2].Failed)

	// DONE: 2 done (from the last workflow phase — Activate)
	assert.Equal(t, 2, phaseFlow.data[3].Done,
		"DONE column should have 2 Done machines from the last phase")
}

// TestRender_StaleEmptyKeyIgnored verifies that the PhaseFlow correctly reads
// the Done count from the last workflow phase even when the stats map contains
// a stale "" key (simulating pre-fix pollution from machines that hadn't started).
func TestRender_StaleEmptyKeyIgnored(t *testing.T) {
	t.Parallel()

	workflowPhases := []phase.Phase{phase.Inspect, phase.Build, phase.Activate}

	spp := stats.New(workflowPhases)
	spp.DeepSet(phase.Activate, stats.Done, xpath.New("flake0/cfg0/m0"))
	// Simulate pollution: add a "" key at the end of the ordered map.
	spp.DeepSet(phase.Phase(""), "", xpath.New("flake0/cfg0/m1"))

	fleetInst := &fleet.Fleet{CacheStatisticsPerPhase: spp}
	phaseFlow := New(fleetInst, colorscheme.DefaultColorScheme(), workflowPhases)

	phaseFlow.Render(200)

	// Even with the "" key in the map, the DONE column should read from
	// the last workflow phase (Activate), not from spp.Last() which would
	// return the "" key.
	assert.Equal(t, 1, phaseFlow.data[3].Done,
		"DONE column should read from last workflow phase, not from stale map keys")

	// Data alignment: exactly len(workflowPhases)+1 entries.
	assert.Len(t, phaseFlow.data, len(workflowPhases)+1,
		"data should have one entry per phase + DONE, regardless of extra map keys")
}

func TestRender_EmptyPhases(t *testing.T) {
	t.Parallel()

	spp := stats.New([]phase.Phase{})
	fleetInst := &fleet.Fleet{CacheStatisticsPerPhase: spp}
	phaseFlow := New(fleetInst, colorscheme.DefaultColorScheme(), []phase.Phase{})

	// Should not panic with empty phases.
	buf := phaseFlow.Render(200)
	assert.NotNil(t, buf)
}

func TestRender_NilStatsPerPhase(t *testing.T) {
	t.Parallel()

	fleetInst := &fleet.Fleet{}
	phaseFlow := New(fleetInst, colorscheme.DefaultColorScheme(), []phase.Phase{phase.Inspect, phase.Build})

	// Should not panic with nil CacheStatisticsPerPhase.
	buf := phaseFlow.Render(200)
	assert.NotNil(t, buf)
}
