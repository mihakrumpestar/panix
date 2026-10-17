package phaseflow

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/theme"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
)

func TestRender_DoneColumnFromLastPhase(t *testing.T) {
	t.Parallel()

	workflowPhases := []phase.Phase{phase.Inspect, phase.Build, phase.Activate}

	spp := stats.New(workflowPhases)
	spp.DeepSet(phase.Activate, stats.Done, xpath.New("flake0/cfg0/m0"))
	spp.DeepSet(phase.Activate, stats.Done, xpath.New("flake0/cfg0/m1"))
	spp.DeepSet(phase.Build, stats.Running, xpath.New("flake0/cfg0/m2"))

	fleetInst := &fleet.Fleet{CacheStatisticsPerPhase: spp}
	phaseFlow := New(fleetInst, theme.DefaultTheme(), workflowPhases)

	phaseFlow.Render(200)

	// The stats map holds one entry per workflow phase plus DONE.
	assert.Len(t, phaseFlow.data, len(workflowPhases)+1,
		"data should have one entry per phase + DONE")

	assert.Equal(t, 0, phaseFlow.data[0].Running)
	assert.Equal(t, 0, phaseFlow.data[0].Failed)

	assert.Equal(t, 1, phaseFlow.data[1].Running)
	assert.Equal(t, 0, phaseFlow.data[1].Failed)

	// Done is read separately for the DONE column.
	assert.Equal(t, 0, phaseFlow.data[2].Running)
	assert.Equal(t, 0, phaseFlow.data[2].Failed)

	assert.Equal(t, 2, phaseFlow.data[3].Done,
		"DONE column should have 2 Done machines from the last phase")
}

// TestRender_StaleEmptyKeyIgnored pins that the DONE count comes from the last
// workflow phase even when the stats map contains a stale "" key.
func TestRender_StaleEmptyKeyIgnored(t *testing.T) {
	t.Parallel()

	workflowPhases := []phase.Phase{phase.Inspect, phase.Build, phase.Activate}

	spp := stats.New(workflowPhases)
	spp.DeepSet(phase.Activate, stats.Done, xpath.New("flake0/cfg0/m0"))
	// Pollution: a "" key at the end of the ordered map.
	spp.DeepSet(phase.Phase(""), "", xpath.New("flake0/cfg0/m1"))

	fleetInst := &fleet.Fleet{CacheStatisticsPerPhase: spp}
	phaseFlow := New(fleetInst, theme.DefaultTheme(), workflowPhases)

	phaseFlow.Render(200)

	// Even with the "" key present, the DONE column reads from the last workflow
	// phase (Activate), not from spp.Last() which would return the "" key.
	assert.Equal(t, 1, phaseFlow.data[3].Done,
		"DONE column should read from last workflow phase, not from stale map keys")

	// Data alignment stays exact regardless of extra map keys.
	assert.Len(t, phaseFlow.data, len(workflowPhases)+1,
		"data should have one entry per phase + DONE, regardless of extra map keys")
}

func TestRender_EmptyPhases(t *testing.T) {
	t.Parallel()

	spp := stats.New([]phase.Phase{})
	fleetInst := &fleet.Fleet{CacheStatisticsPerPhase: spp}
	phaseFlow := New(fleetInst, theme.DefaultTheme(), []phase.Phase{})

	buf := phaseFlow.Render(200)
	assert.NotNil(t, buf)
}

func TestRender_NilStatsPerPhase(t *testing.T) {
	t.Parallel()

	fleetInst := &fleet.Fleet{}
	phaseFlow := New(fleetInst, theme.DefaultTheme(), []phase.Phase{phase.Inspect, phase.Build})

	buf := phaseFlow.Render(200)
	assert.NotNil(t, buf)
}
