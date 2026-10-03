package executioner

import (
	"context"
	"testing"

	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClone_SharesConfRegistersIntoSharedPhaseLog pins the clone seam the
// activation guard's confirm loop relies on: same configuration, independent
// per-instance state, and execs that land in the shared phase log.
func TestClone_SharesConfRegistersIntoSharedPhaseLog(t *testing.T) {
	t.Parallel()

	phaseLog := phaselogs.NewPhaseLog()
	exc := NewExecutioner(ExecutionerConf{
		Ctx:          context.Background(),
		Xpath:        xpath.New("test"),
		Phase:        phase.Activate,
		PhaseLog:     phaseLog,
		DryRun:       true,
		OnUpdateHook: func() {},
	})

	clone := exc.Clone()

	require.NotNil(t, clone)
	assert.True(t, clone.DryRun(), "the clone shares the dry-run configuration")
	assert.NotNil(t, clone.Context(), "the clone always exposes a usable context")

	err := clone.Exec("cloned step", "running", "failed", []string{"echo", "hi"})
	require.NoError(t, err)

	_, found := phaseLog.CommandLogs.Last()
	assert.True(t, found, "the clone's exec registers in the shared phase log")

	// The clone's per-instance xpath appends the phase independently.
	assert.NotEmpty(t, exc.phaseXpath.String())
	assert.NotSame(t, exc, clone)
}
