package secrets

import (
	"context"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
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

// TestSecretTransferLabel pins the non-sensitive failure labels: file sources
// keep their path while command-only sources get a stable index that cannot
// contain the command text.
func TestSecretTransferLabel(t *testing.T) {
	t.Parallel()

	const command = "printf super-secret-command-text"

	assert.Equal(t, "/tmp/age.key", secretTransferLabel(0, attributes.TransferSource{
		LocalPath: "/tmp/age.key",
		Command:   command,
	}))

	label := secretTransferLabel(1, attributes.TransferSource{Command: command})
	assert.Equal(t, "command source #2", label)
	assert.NotContains(t, label, command)
}

// TestRunPhaseErrorDoesNotLeakCommand errors a command-sourced transfer and
// pins that neither the returned error nor the command log embeds the command
// text or the sh -u -c argv. The command is plain text, not octal-escaped.
func TestRunPhaseErrorDoesNotLeakCommand(t *testing.T) {
	t.Parallel()

	mach := newSecretTestMachine(t)

	const command = "printf super-secret-command-text; exit 1"

	remotePath := t.TempDir() + "/secret.bin"

	mach.Secrets = []attributes.TransferSource{{
		Command:    command,
		RemotePath: remotePath,
	}}

	leaf := &fleet.FleetLeaf{
		Machine:     mach,
		Installable: &installable.Installable{},
	}

	phaseLog := phaselogs.NewPhaseLog()

	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		Timeout:      10 * time.Second,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        phase.Secrets,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	err := (Handler{}).RunPhase(exc, leaf)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "command source #1")
	assert.NotContains(t, err.Error(), "super-secret-command-text")

	last, ok := phaseLog.CommandLogs.Last()
	require.True(t, ok)
	assert.Equal(t, "command source "+remotePath, last.Command.String())
	assert.NotContains(t, last.Command.String(), "super-secret-command-text")
	assert.NotContains(t, last.Command.String(), "sh -u")
}

func newSecretTestMachine(t *testing.T) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
	}
	client := &ssh.SSHClient{Hostname: "local-test"}
	require.NoError(t, client.Init("local-test", "local-test", nixver.Info{}))

	mach.SSH = *client
	mach.SudoProgram = attributes.SudoProgram("sudo")
	mach.MetaInspect.Store(&machine.MetaInspect{IsRoot: true})

	return mach
}
