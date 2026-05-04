package tui

import (
	"context"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/tui/phasestatus"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require")

func TestStartResetableWorkflow_ReturnsWorkflowDoneMsg(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	cmd := mdl.startResetableWorkflow()
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(workflowDoneMsg)
	require.True(t, ok, "expected workflowDoneMsg, got %T", msg)
}

func TestWorkflowDoneMsg_WithErr_SetsModelError(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	workflowErr := errors.New("workflow completed with 1 machine(s) failed")
	msg := workflowDoneMsg{err: workflowErr}

	_ = mdl.Update(msg)

	assert.False(t, mdl.quitting, "TUI should stay open on workflowDoneMsg even with error")
	assert.Equal(t, workflowErr, mdl.err, "m.err should be set from workflowDoneMsg.err")
}

func TestWorkflowDoneMsg_WithoutErr_NoError(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	msg := workflowDoneMsg{}

	_ = mdl.Update(msg)

	assert.False(t, mdl.quitting)
	assert.NoError(t, mdl.err)
}

func TestErrMsg_AlwaysQuits(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	msg := errMsg{err: errors.New("something fatal")}

	_ = mdl.Update(msg)

	assert.True(t, mdl.quitting, "errMsg should always quit the TUI")
	assert.Error(t, mdl.err)
}

func TestRestartWorkflow_ClearsError(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	mdl.err = errors.New("workflow completed with 1 machine(s) failed")

	cmd := mdl.restartWorkflow()
	require.NotNil(t, cmd, "restartWorkflow should return a cmd to start a new workflow")

	assert.NoError(t, mdl.err, "restartWorkflow should clear m.err")
}

func TestRestartWorkflow_IgnoresCancelErrors(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	// Start a workflow first so there's something to cancel
	startCmd := mdl.startResetableWorkflow()
	_ = startCmd()

	// The workflow was started; now restart it.
	// Cancel() always returns context.Canceled, but we should
	// not care about ANY error from cancel on restart.
	restartCmd := mdl.restartWorkflow()
	require.NotNil(t, restartCmd, "restart should proceed even if cancel errors")
	assert.NoError(t, mdl.err, "restart should clear m.err regardless of cancel errors")
}

func TestRestartWorkflow_NoExistingWorkflow(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)
	mdl.resetable.Store(nil)

	cmd := mdl.restartWorkflow()
	require.NotNil(t, cmd, "restart should start a new workflow even without an existing one")
	assert.NoError(t, mdl.err)
}

func TestHandleQuit_DetectsFailedMachinesFromFleetState(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)
	require.NoError(t, mdl.err, "no error initially")

	// Mark a machine as failed in fleet state
	for _, fleetLeaf := range mdl.conf.Fleet.AllMachines() {
		fleetLeaf.Machine.State.Store(&machine.State{
			Status: stats.Failed,
		})

		break
	}

	_ = mdl.handleQuit()

	assert.True(t, mdl.quitting)
	assert.ErrorIs(t, mdl.err, errMachinesFailed, "handleQuit should set m.err from fleet state when machines are failed")
}

func TestHandleQuit_PreservesExistingError(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)
	existingErr := errors.New("workflow completed with 2 machine(s) failed")
	mdl.err = existingErr

	_ = mdl.handleQuit()

	assert.True(t, mdl.quitting)
	assert.Equal(t, existingErr, mdl.err, "handleQuit should preserve already-set m.err")
}

func TestHandleQuit_NoErrorWhenAllSucceeded(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	for _, fleetLeaf := range mdl.conf.Fleet.AllMachines() {
		fleetLeaf.Machine.State.Store(&machine.State{
			Status: stats.Done,
		})
	}

	_ = mdl.handleQuit()

	assert.True(t, mdl.quitting)
	assert.NoError(t, mdl.err, "handleQuit should not set m.err when all machines succeeded")
}

func newTestModel(t *testing.T) *model {
	t.Helper()

	conf := makeTestConfig()
	conf.Flags.DryRun = true

	mdl := &model{
		ctx:         context.Background(),
		conf:        conf,
		dimensions:  &viewports.Dimensions{Width: 200, Height: 80},
		header:      header.New(false, config.Snapshot{}),
		spinners:    spinners.NewSpinners(),
		statsTable:  statstable.NewStatsTable(conf.Fleet, conf.ColorScheme),
		phaseStatus: phasestatus.NewPhaseStatus(conf.Fleet, conf.ColorScheme, conf.Phases),
	}
	mdl.footer = footer.New(mdl.keyDefs(), conf, conf.ColorScheme)

	return mdl
}

func makeTestConfig() *config.Config {
	mach := &machine.Machine{
		State:       atomicpointer.New[machine.State](),
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
		Logs:        logs.New(),
	}
	mach.State.Store(&machine.State{})

	machinesMap := atomicorderedmap.New[string, *machine.Machine]()
	machinesMap.Set("m0", mach)

	cfg := &configuration.Configuration{}
	cfg.Logs = logs.New()
	cfg.Machines = machinesMap

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()
	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Configurations = atomicorderedmap.New[string, *configuration.Configuration]()
	flakeObj.Configurations.Set("cfg0", cfg)
	flakesMap.Set("flake0", flakeObj)

	colorScheme := colorscheme.DefaultColorScheme()

	return &config.Config{
		ColorScheme: colorScheme,
		Fleet: &fleet.Fleet{
			Flakes: flakesMap,
			Logs:   logs.New(),
		},
		Phases: []phase.Phase{phase.Inspect, phase.Build, phase.Activate},
	}
}
