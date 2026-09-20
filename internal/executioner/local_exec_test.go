package executioner

import (
	"context"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLocalExecMachine builds an ssh-free local machine; commands run through
// shellStream on a PTY.
func newLocalExecMachine(t *testing.T) *machine.Machine {
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

func newLocalExecutioner(t *testing.T, mach *machine.Machine, mutators ...func(*ExecutionerConf)) (*Executioner, *phaselogs.PhaseLog) {
	t.Helper()

	conf := ExecutionerConf{
		Ctx:          context.Background(),
		Timeout:      10 * time.Second,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Bootstrap,
		PhaseLog:     phaselogs.NewPhaseLog(),
		OnUpdateHook: func() {},
	}

	for _, mutate := range mutators {
		mutate(&conf)
	}

	return NewExecutioner(conf), conf.PhaseLog
}

// Pins the local half of the transport-agnostic quoting model: space-bearing
// argv elements arrive verbatim (the remote half is covered by the e2e suite).
func TestExec_LocalTransportSpaceBearingArgs(t *testing.T) {
	t.Parallel()

	mach := newLocalExecMachine(t)
	exc, phaseLog := newLocalExecutioner(t, mach)

	err := exc.Exec(
		"echo args",
		"echoing",
		"echo failed",
		[]string{"sh", "-c", `printf '%s' "$@"`, "--", "a b c", "it's"},
	)
	require.NoError(t, err)

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)

	out := last.Output.String()
	assert.Contains(t, out, "a b c")
	assert.Contains(t, out, "it's")
}

// Pins that sh -c hook semantics work end-to-end through the local transport,
// without a remote shell.
func TestExec_LocalTransportHooksSemantics(t *testing.T) {
	t.Parallel()

	mach := newLocalExecMachine(t)
	exc, phaseLog := newLocalExecutioner(t, mach)

	err := exc.Exec(
		"hook",
		"running hook",
		"hook failed",
		[]string{"sh", "-c", "echo hook-ok"},
	)
	require.NoError(t, err)

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Contains(t, last.Output.String(), "hook-ok")
}
