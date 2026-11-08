package workflow

import (
	"context"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/structdefault"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHardwarePhaseMachine builds a machine that is local to this process, so the
// recorded command line carries no ssh wrapper. Every command stays dry-run: the
// workflow below hardcodes it.
func newHardwarePhaseMachine(t *testing.T, name, hardwarePath string) *machine.Machine {
	t.Helper()

	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
		Logs:        logs.New(),
	}

	client := &ssh.SSHClient{Hostname: name}
	require.NoError(t, client.Init(name, name, nixver.Info{}))

	mach.SSH = *client
	mach.SudoProgram = attributes.SudoProgram("sudo")
	mach.Xpath = xpath.New("flake", "nixosConfigurations", "cfg", name)
	mach.HardwareConfig.Path = hardwarePath
	structdefault.Apply(&mach.Attributes)

	return mach
}

// TestWorkflow_HardwareConfigPhase pins the command's phase plan end to end: the
// Inspect phase runs first and leaves the generation to the hardware-config
// phase, which runs the generator once. Dry-run is hardcoded so no command can
// reach the machine that runs the test.
func TestWorkflow_HardwareConfigPhase(t *testing.T) {
	t.Parallel()

	mach := newHardwarePhaseMachine(t, "hw-phase", "/repo/nix/hardware-configuration.nix")
	fake := testutil.NewFaker()

	conf := &config.Config{
		Flags:  flags.Flags{WorkflowFlags: flags.WorkflowFlags{DryRun: true}},
		Phases: []phase.Phase{phase.Inspect, phase.HardwareConfig},
		Fleet:  fake.Fleet(fake.Flake(fake.Installable(mach))),
	}
	require.True(t, conf.Flags.DryRun)

	workflowInst, err := NewWorkflow(context.Background(), conf)
	require.NoError(t, err)
	require.NoError(t, workflowInst.StartWorkflow())

	inspectLog, ok := mach.Logs.PhaseLogs.Get(phase.Inspect)
	require.True(t, ok, "inspect must log to its own phase")

	hardwareLog, ok := mach.Logs.PhaseLogs.Get(phase.HardwareConfig)
	require.True(t, ok, "the generation must log to the hardware-config phase")

	assert.NotEmpty(t, testutil.CommandLines(t, inspectLog))
	assert.Equal(t,
		[]string{"nixos-generate-config --show-hardware-config --no-filesystems"},
		testutil.CommandLines(t, hardwareLog),
	)
}
