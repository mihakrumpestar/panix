package workflow

import (
	"context"
	"fmt"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

type WorkflowExecutor struct {
	ctx      context.Context
	cancel   context.CancelFunc
	cfg      *config.Config
	phases   []workflow_definition.WorkflowPhase
	metadata *ExecutionResult
}

type ExecutionResult struct {
	FlakesMetadata []FlakeMetadata
}

type FlakeMetadata struct {
	Name            string
	ConfigsMetadata []ConfigMetadata
}

type ConfigMetadata struct {
	Name             string
	BuildOutputPath  string
	MachinesMetadata []MachineMetadata
}

type MachineMetadata struct {
	Name string
}

func NewWorkflowExecutor(ctx context.Context, cfg *config.Config, phases []workflow_definition.WorkflowPhase) (*WorkflowExecutor, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.Global.Timeout)

	filteredPhases := make([]workflow_definition.WorkflowPhase, 0)

	for _, phase := range phases {
		if !slices.Contains(cfg.Global.SkipPhases, phase) {
			filteredPhases = append(filteredPhases, phase)
		}
	}

	if len(filteredPhases) == 0 {
		cancel()
		return nil, fmt.Errorf("No phases left after filtering")
	}

	// Initialize metadata structure
	metadata := &ExecutionResult{
		FlakesMetadata: make([]FlakeMetadata, 0),
	}

	return &WorkflowExecutor{
		ctx:      ctx,
		cancel:   cancel,
		phases:   phases,
		cfg:      cfg,
		metadata: metadata,
	}, nil
}

// Execute always returns ExecutionResults for debugging, even on error
func (w *WorkflowExecutor) Execute() (*ExecutionResult, error) {
	defer w.cancel()

	_, err := w.executePhase(w.phases)
	if err != nil {
		return w.metadata, err
	}

	return w.metadata, nil
}

func (w *WorkflowExecutor) executePhase(currentPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	if len(currentPhases) == 0 {
		return &ExecutionResult{}, nil
	}

	currentPhase, nextPhases := currentPhases[0], currentPhases[1:]

	if w.cfg.Global.Verbose {
		fmt.Printf("starting phase: %s\n", currentPhase)
	}

	switch currentPhase {
	case workflow_definition.PhasePreflight:
		return w.executePreflight(nextPhases)
	case workflow_definition.PhaseBuild:
		return w.executeBuild(nextPhases)
	case workflow_definition.PhaseBootstrap:
		return w.executeBootstrap(nextPhases)
	case workflow_definition.PhaseTransfer:
		return w.executeTransfer(nextPhases)
	case workflow_definition.PhaseSecrets:
		return w.executeSecrets(nextPhases)
	case workflow_definition.PhaseActivate:
		return w.executeActivatePhase(nextPhases)
	case workflow_definition.PhaseStatus:
		return w.executeStatus(nextPhases)
	default:
		return nil, fmt.Errorf("unknown phase: %s", currentPhase)
	}
}

type machineInfo struct {
	flakeName   string
	configName  string
	machineName string
	machine     *config.Machine
}
