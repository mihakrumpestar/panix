package phaseops

import (
	"context"
	"os/user"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realExecutionerFor builds a non-dry-run executioner on the local machine, for
// tests that execute actual commands through the local PTY transport.
func realExecutionerFor(t *testing.T, mach *machine.Machine) (*executioner.Executioner, *phaselogs.PhaseLog) {
	t.Helper()

	phaseLog := phaselogs.NewPhaseLog()
	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		Timeout:      10 * time.Second,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Inspect,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	return exc, phaseLog
}

// TestRefreshSuperuser_LocalProbe runs the real probe through the local
// transport and pins that IsRoot matches the actual runtime identity; the stored
// value starts wrong, so the assertion proves the probe updated it.
func TestRefreshSuperuser_LocalProbe(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, false)
	// Force a stale wrong value: the probe must overwrite it with reality.
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: !isRunningRoot(t)})

	exc, phaseLog := realExecutionerFor(t, mach)

	require.NoError(t, RefreshSuperuser(exc, mach))

	mi := mach.MetaInspect.Load()
	require.NotNil(t, mi)
	assert.Equal(t, isRunningRoot(t), mi.IsRoot,
		"IsRoot must reflect the real running user")
	assert.True(t, mi.IsRootProbed, "a real probe must set IsRootProbed")

	assert.NotEmpty(t, testutil.CommandLines(t, phaseLog))
}

// TestRefreshSuperuser_DryRunKeepsInspectedValue pins that a dry-run re-probe
// must not clobber an IsRoot already probed by a real Inspect.
func TestRefreshSuperuser_DryRunKeepsInspectedValue(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, false)
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: false, IsRootProbed: true})

	exc, _ := testutil.NewDryRunExecutioner(t, mach, phase.Activate)
	require.NoError(t, RefreshSuperuser(exc, mach))

	mi := mach.MetaInspect.Load()
	require.NotNil(t, mi)
	assert.False(t, mi.IsRoot, "dry-run re-probe must not overwrite the real inspected value with true")
}

// TestRefreshSuperuser_DryRunWithoutProbeDefaultsRoot pins that a fresh dry-run
// defaults IsRoot to true so previews do not fail elevation-dependent paths.
func TestRefreshSuperuser_DryRunWithoutProbeDefaultsRoot(t *testing.T) {
	t.Parallel()

	mach := newLocalTransferMachine(t, false)
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: false, IsRootProbed: false})

	exc, _ := testutil.NewDryRunExecutioner(t, mach, phase.Inspect)
	require.NoError(t, RefreshSuperuser(exc, mach))

	mi := mach.MetaInspect.Load()
	require.NotNil(t, mi)
	assert.True(t, mi.IsRoot, "fresh dry-run defaults to root placeholder")
	assert.False(t, mi.IsRootProbed)
}

func isRunningRoot(t *testing.T) bool {
	t.Helper()

	current, err := user.Current()
	require.NoError(t, err)

	return current.Uid == "0"
}
