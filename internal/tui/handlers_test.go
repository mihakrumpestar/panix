package tui

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/tui/footer"
	"github.com/mihakrumpestar/panix/internal/tui/header"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartWorkflowCmd_ReturnsWorkflowDoneMsg(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	workflow, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	cmd := workflowRunCmd(workflow)
	require.NotNil(t, cmd)

	msg := cmd()
	doneMsg, ok := msg.(workflowDoneMsg)
	require.True(t, ok, "expected workflowDoneMsg, got %T", msg)
	assert.Equal(t, workflow, doneMsg.workflow)
}

func TestWorkflowDoneMsg_WithErr_SetsModelError(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	workflow, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = workflow

	workflowErr := errors.New("workflow completed with 1 machine(s) failed")
	msg := workflowDoneMsg{workflow: workflow, err: workflowErr}

	_ = mdl.Update(msg)

	assert.False(t, mdl.quitting, "TUI should stay open on workflowDoneMsg even with error")
	assert.Equal(t, workflowErr, mdl.err, "m.err should be set from workflowDoneMsg.err")
}

func TestWorkflowDoneMsg_WithoutErr_NoError(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	w, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = w

	msg := workflowDoneMsg{workflow: w}

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

	cmd := mdl.workflowRestartCmd()
	require.NotNil(t, cmd, "restartWorkflow should return a cmd to start a new workflow")

	assert.NoError(t, mdl.err, "restartWorkflow should clear m.err")
}

func TestRestartWorkflow_IgnoresCancelErrors(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	// Set up a workflow and start it so Cancel() can complete
	w, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = w

	go func() { _ = w.StartWorkflow() }()

	// Wait briefly for the workflow to be running
	time.Sleep(10 * time.Millisecond)

	// Cancel blocks until StartWorkflow finishes, but the cancelled
	// context should make it complete quickly.
	restartCmd := mdl.workflowRestartCmd()
	require.NotNil(t, restartCmd, "restart should proceed even if cancel errors")
	assert.NoError(t, mdl.err, "restart should clear m.err regardless of cancel errors")
}

func TestRestartWorkflow_NoExistingWorkflow(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)
	mdl.workflow = nil

	cmd := mdl.workflowRestartCmd()
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

func TestHandleQuit_RunningWorkflow_ShowsHintAndWaits(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	workflow, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = workflow

	cmd := mdl.handleQuit()

	// Graceful quit: the notification cmd is returned, not a quit command.
	require.NotNil(t, cmd, "graceful quit should show the quitting notification")

	msg := cmd()
	_, isQuit := msg.(zeroterm.QuitMsg)
	assert.False(t, isQuit, "graceful quit should not return a quit command")
	assert.True(t, mdl.quitting)

	// The persistent quitting hint is rendered in the footer.
	rendered := mdl.footer.Render(mdl.dimensions.Width)
	require.NotNil(t, rendered)

	hintVisible := false

	for i := range rendered.Len() {
		if bytes.Contains(rendered.Line(i), []byte("Quitting, waiting for running commands to finish")) {
			hintVisible = true

			break
		}
	}

	assert.True(t, hintVisible, "quit hint should be visible in the rendered footer")

	// Once the workflow finishes, the done message completes the quit and
	// resolves the failed-machines error.
	for _, fleetLeaf := range mdl.conf.Fleet.AllMachines() {
		fleetLeaf.Machine.State.Store(&machine.State{
			Status: stats.Failed,
		})

		break
	}

	doneCmd := mdl.Update(workflowDoneMsg{workflow: workflow})
	require.NotNil(t, doneCmd)

	doneMsg := doneCmd()
	_, isQuit = doneMsg.(zeroterm.QuitMsg)
	assert.True(t, isQuit, "workflowDoneMsg while quitting should return QuitCmd")
	assert.ErrorIs(t, mdl.err, errMachinesFailed)
}

func TestHandleForceQuit_ReturnsQuitCmdAndSetsFailedMachinesError(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	workflow, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = workflow

	for _, fleetLeaf := range mdl.conf.Fleet.AllMachines() {
		fleetLeaf.Machine.State.Store(&machine.State{
			Status: stats.Failed,
		})

		break
	}

	cmd := mdl.handleForceQuit()
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(zeroterm.QuitMsg)
	assert.True(t, ok, "force quit should return QuitCmd")

	assert.True(t, mdl.quitting)
	assert.ErrorIs(t, mdl.err, errMachinesFailed, "force quit should set m.err from fleet state when machines are failed")
}

func TestWorkflowDoneMsg_Quitting_ReturnsQuitCmd(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	workflow, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = workflow
	mdl.quitting = true

	cmd := mdl.Update(workflowDoneMsg{workflow: workflow})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(zeroterm.QuitMsg)
	assert.True(t, ok, "workflowDoneMsg while quitting should return QuitCmd")
}

func TestKeys_DisabledWhileQuitting(t *testing.T) {
	t.Parallel()

	mdl := newTestModel(t)

	workflow, err := workflow.NewWorkflow(mdl.ctx, mdl.conf)
	require.NoError(t, err)

	mdl.workflow = workflow
	mdl.quitting = true

	// While quitting, all keys except ctrl+q (force quit) are dead at the
	// dispatch level, including q itself.
	for _, key := range []string{"q", "r", "ctrl+r", "s", "h", "a", "c", "ctrl+c", "m"} {
		assert.Nil(t, mdl.HandleKeyInput(zeroterm.KeyPressMsg{Key: key}), "key %q should be disabled while quitting", key)
	}

	// ctrl+q force quit still works while quitting.
	cmd := mdl.HandleKeyInput(zeroterm.KeyPressMsg{Key: "ctrl+q"})
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(zeroterm.QuitMsg)
	assert.True(t, ok, "ctrl+q should force quit even while quitting")

	// The rendered keymap reflects what actually works: disabled keys are
	// hidden, only the quit keys remain visible.
	rendered := mdl.footer.Render(mdl.dimensions.Width)
	require.NotNil(t, rendered)

	footerText := &bytes.Buffer{}
	for i := range rendered.Len() {
		footerText.Write(rendered.Line(i))
	}

	assert.Contains(t, footerText.String(), "force quit")
	assert.NotContains(t, footerText.String(), "retry")
	assert.NotContains(t, footerText.String(), "restart")
	assert.NotContains(t, footerText.String(), "snapshot")
	assert.NotContains(t, footerText.String(), "copy")
}

func newTestModel(t *testing.T) *model {
	t.Helper()

	conf := makeTestConfig()
	conf.Flags.DryRun = true

	dims := &viewports.Dimensions{Width: 200, Height: 80}

	tableS := conf.ColorScheme.Table

	mdl := &model{
		ctx:        context.Background(),
		conf:       conf,
		dimensions: dims,
		header:     header.New(false, config.Snapshot{}, nil),
		spinners:   spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval),
		statsTable: statstable.New(conf.Fleet, conf.ColorScheme),
		phaseFlow:  phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases),
		viewports: viewports.New(dims, 0, tableS.Border,
			tableS.SelectionHighlightBackground, tableS.SelectionHighlightBorder,
		),
	}
	mdl.footer = footer.New(mdl.keyDefs(), conf.ColorScheme)

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

	inst := &installable.Installable{}
	inst.Logs = logs.New()
	inst.Machines = machinesMap

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()
	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Installables = atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]]()

	attrMap := atomicorderedmap.New[string, *installable.Installable]()
	attrMap.Set("cfg0", inst)
	flakeObj.Installables.Set("nixosConfigurations", attrMap)
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
