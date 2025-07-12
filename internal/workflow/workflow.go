package workflow

import (
	"context"
	"fmt"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
)

type WorkflowPhase string

const (
	PhasePreflight WorkflowPhase = "preflight"
	PhaseBootstrap WorkflowPhase = "bootstrap"
	PhaseSecrets   WorkflowPhase = "secrets"
	PhaseBuild     WorkflowPhase = "build"
	PhaseTransfer  WorkflowPhase = "transfer"
	PhaseActivate  WorkflowPhase = "activate"
	PhaseStatus    WorkflowPhase = "status"
)

type WorkflowExecutor struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	phases []WorkflowPhase
	opts   WorkflowOptions
}

type WorkflowOptions struct {
	DryRun     bool
	Verbose    bool
	SkipPhases []WorkflowPhase
}

type ExecutionResult struct {
	FlakesMetadata []FlakeMetadata
}

type FlakeMetadata struct {
	ConfigsMetadata []ConfigMetadata
}

type ConfigMetadata struct {
	MachinesMetadata []MachineMetadata
}

type MachineMetadata struct {
}

func NewWorkflowExecutor(ctx context.Context, cfg *config.Config, phases []WorkflowPhase, opts WorkflowOptions) (*WorkflowExecutor, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.Global.Timeout)

	filteredPhases := make([]WorkflowPhase, 0)

	for _, phase := range phases {
		if !slices.Contains(opts.SkipPhases, phase) {
			filteredPhases = append(filteredPhases, phase)
		}
	}

	if len(filteredPhases) == 0 {
		return nil, fmt.Errorf("No phases left after filtering")
	}

	return &WorkflowExecutor{
		ctx:    ctx,
		cancel: cancel,
		phases: phases,
		cfg:    cfg,
		opts:   opts,
	}, nil
}

// Execute always returns ExecutionResults for debugging, even on error
func (w *WorkflowExecutor) Execute() (*ExecutionResult, error) {
	defer w.cancel()

	executionResult, err := w.executePhase(w.phases)
	if err != nil {
		return executionResult, err
	}

	return executionResult, nil
}

func (w *WorkflowExecutor) executePhase(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	currentPhase, nextPhases := currentPhases[0], currentPhases[1:]

	if w.opts.Verbose {
		fmt.Printf("starting phase: %s\n", currentPhase)
	}

	switch currentPhase {
	case PhasePreflight:
		return w.executePreflight(nextPhases)
	case PhaseBuild:
		// Branches Flakes, Configurations
		return w.executeBuild(nextPhases)
		// Branches Machines
	case PhaseBootstrap:
		return w.executeBootstrap(nextPhases)
	case PhaseTransfer:
		return w.executeTransfer(nextPhases)
	case PhaseSecrets:
		return w.executeSecrets(nextPhases)
	case PhaseActivate:
		return w.executeActivate(nextPhases)
	// Runs seperate
	case PhaseStatus:
		return w.executeStatus(nextPhases)
	default:
		return nil, fmt.Errorf("unknown phase: %s", currentPhase)
	}
}

// Helpers

func (w *WorkflowExecutor) executeBranching(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	something
	if err != nil {
		if w.cfg.Global.RequireAllSuccess {
			return nil, err
		}
		fmt.Printf("Warning: failed but continuing: %v\n", err)
	}

	return
}

func (w *WorkflowExecutor) executeBootstrap(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	// TODO: Implement nixos-anywhere bootstrap
	return nil, nil
}

func (w *WorkflowExecutor) executeSecrets(currentPhases []WorkflowPhase) (*ExecutionResult, error) {
	// TODO: Implement secrets deployment
	return nil, nil
}
