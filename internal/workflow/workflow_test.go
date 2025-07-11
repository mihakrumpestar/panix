package workflow

import (
	"context"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowExecution(t *testing.T) {
	cfg := &config.GlobalConfig{
		RequireAllSuccess: true,
	}
	machines := []config.MachineConfig{
		{Name: "machine1"},
		{Name: "machine2"},
	}

	w := NewWorkflowExecutor(context.Background(), cfg, machines)

	opts := WorkflowOptions{
		Phases: []WorkflowPhase{PhasePreflight, PhaseBuild},
	}

	err := w.Execute(opts)
	assert.NoError(t, err)
}
