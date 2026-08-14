package workflow

import (
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
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartWorkflow_CancelledContextNoFailedCount(t *testing.T) {
	t.Parallel()

	conf := makeWorkflowTestConfig()
	conf.Flags.DryRun = true

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	workflowInst, err := NewWorkflow(ctx, conf)
	require.NoError(t, err)

	err = workflowInst.StartWorkflow()
	if err != nil {
		assert.NotContains(t, err.Error(), "machine(s) failed",
			"cancelled workflow should not report failed machines")
	}
}

func TestStartWorkflow_CancelledDuringRun(t *testing.T) {
	t.Parallel()

	conf := makeWorkflowTestConfig()
	conf.Flags.DryRun = true

	ctx, cancel := context.WithCancel(context.Background())

	workflowInst, err := NewWorkflow(ctx, conf)
	require.NoError(t, err)

	// Cancel after creating the workflow to simulate a restart scenario
	cancel()

	err = workflowInst.StartWorkflow()
	if err != nil {
		assert.NotContains(t, err.Error(), "machine(s) failed",
			"cancelled workflow should not report failed machine count")
	}
}

func TestCancelAsync_DoesNotBlock(t *testing.T) {
	t.Parallel()

	conf := makeWorkflowTestConfig()
	conf.Flags.DryRun = true

	workflowInst, err := NewWorkflow(context.Background(), conf)
	require.NoError(t, err)

	// CancelAsync on a not-yet-started workflow must return immediately,
	// unlike Cancel() which would block forever on <-w.done.
	done := make(chan struct{})

	go func() {
		workflowInst.CancelAsync()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("CancelAsync blocked")
	}
}

func TestDone_ClosedAfterStartWorkflowReturns(t *testing.T) {
	t.Parallel()

	conf := makeWorkflowTestConfig()
	conf.Flags.DryRun = true

	workflowInst, err := NewWorkflow(context.Background(), conf)
	require.NoError(t, err)

	select {
	case <-workflowInst.Done():
		t.Fatal("Done closed before StartWorkflow returned")
	default:
	}

	go func() { _ = workflowInst.StartWorkflow() }()

	select {
	case <-workflowInst.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed after StartWorkflow returned")
	}
}

func makeWorkflowTestConfig() *config.Config {
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

	return &config.Config{
		ColorScheme: colorscheme.DefaultColorScheme(),
		Fleet: &fleet.Fleet{
			Flakes: flakesMap,
		},
		Phases: []phase.Phase{phase.Inspect, phase.Build, phase.Activate},
	}
}
