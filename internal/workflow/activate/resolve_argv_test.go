package activate

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newResolveMachine builds a LOCAL machine for resolveCommandPath tests:
// real execution through the local PTY transport.
func newResolveMachine(t *testing.T) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: true})

	return mach
}

// newRealExecutioner builds a REAL (non-dry-run) executioner for tests
// that execute actual commands.
func newRealExecutioner(t *testing.T, mach *machine.Machine) *executioner.Executioner {
	t.Helper()

	phaseLog := phaselogs.NewPhaseLog()

	return executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		Timeout:      10 * time.Second,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Activate,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})
}

// Existing commands resolve to an absolute path, missing ones fail with
// the not-found message, absolute input passes through untouched.
func TestResolveCommandPath_RealResolution(t *testing.T) {
	t.Parallel()

	mach := newResolveMachine(t)
	exc := newRealExecutioner(t, mach)

	resolved, err := resolveCommandPath(exc, "sh")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resolved, "/"), "existing command must resolve to an absolute path, got %q", resolved)

	passed, err := resolveCommandPath(exc, "/usr/bin/special-install")
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/special-install", passed)

	_, err = resolveCommandPath(exc, "panix-definitely-not-a-command")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found on PATH on the target")
}

// Dry run previews the bare command name and runs no probe.
func TestResolveCommandPath_DryRunKeepsBareName(t *testing.T) {
	t.Parallel()

	mach := newResolveMachine(t)

	phaseLog := phaselogs.NewPhaseLog()
	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		DryRun:       true,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Activate,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	resolved, err := resolveCommandPath(exc, "nixos-install")
	require.NoError(t, err)
	assert.Equal(t, "nixos-install", resolved)
}
