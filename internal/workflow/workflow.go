package workflow

import (
	"context"
	"fmt"
	"sync"

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
	config   *config.Global
	machines []config.MachineConfig
	ctx      context.Context
	cancel   context.CancelFunc
}

type ExecutionResult struct {
	Machine config.MachineConfig
	Phase   WorkflowPhase
	Error   error
	Output  string
}

type WorkflowOptions struct {
	DryRun    bool
	Verbose   bool
	Phases    []WorkflowPhase
	SkipPhase map[WorkflowPhase]bool
}

func NewWorkflowExecutor(ctx context.Context, cfg *config.Global, machines []config.MachineConfig) *WorkflowExecutor {
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)

	return &WorkflowExecutor{
		config:   cfg,
		machines: machines,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (w *WorkflowExecutor) Execute(opts WorkflowOptions) error {
	defer w.cancel()

	currentMachines := w.machines

	for _, phase := range opts.Phases {
		if opts.SkipPhase[phase] {
			continue
		}

		if opts.Verbose {
			fmt.Printf("Starting phase: %s\n", phase)
		}

		var err error
		currentMachines, err = w.executePhase(phase, opts, currentMachines)
		if err != nil {
			if w.config.RequireAllSuccess {
				return fmt.Errorf("phase %s failed: %w", phase, err)
			}
			fmt.Printf("Warning: phase %s failed but continuing: %v\n", phase, err)
		}
	}

	return nil
}

func (w *WorkflowExecutor) executePhase(phase WorkflowPhase, opts WorkflowOptions, machines []config.MachineConfig) ([]config.MachineConfig, error) {
	var wg sync.WaitGroup
	resultChan := make(chan ExecutionResult, len(machines))

	for _, machine := range machines {
		wg.Add(1)
		go func(m config.MachineConfig) {
			defer wg.Done()
			select {
			case <-w.ctx.Done():
				resultChan <- ExecutionResult{
					Machine: m,
					Phase:   phase,
					Error:   w.ctx.Err(),
				}
			default:
				result := w.executeMachinePhase(m, phase, opts)
				resultChan <- result
			}
		}(machine)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var failures []ExecutionResult
	var nextMachines []config.MachineConfig
	for result := range resultChan {
		if opts.Verbose {
			if result.Error == nil {
				fmt.Printf("✓ %s: %s completed\n", result.Machine.Name, result.Phase)
			} else {
				fmt.Printf("✗ %s: %s failed: %v\n", result.Machine.Name, result.Phase, result.Error)
			}
		}

		if result.Error != nil {
			failures = append(failures, result)
		}
		nextMachines = append(nextMachines, result.Machine)
	}

	if len(failures) > 0 {
		return nextMachines, fmt.Errorf("%d machines failed in phase %s", len(failures), phase)
	}

	return nextMachines, nil
}

func (w *WorkflowExecutor) executeMachinePhase(machine config.MachineConfig, phase WorkflowPhase, opts WorkflowOptions) ExecutionResult {
	if opts.DryRun {
		return ExecutionResult{
			Machine: machine,
			Phase:   phase,
			Output:  fmt.Sprintf("DRY RUN: would execute %s for %s", phase, machine.Name),
		}
	}

	switch phase {
	case PhasePreflight:
		return w.executePreflight(machine)
	case PhaseBootstrap:
		return w.executeBootstrap(machine)
	case PhaseSecrets:
		return w.executeSecrets(machine)
	case PhaseBuild:
		return w.executeBuild(machine)
	case PhaseTransfer:
		return w.executeTransfer(machine)
	case PhaseActivate:
		return w.executeActivate(machine)
	case PhaseStatus:
		return w.executeStatus(machine)
	default:
		return ExecutionResult{
			Machine: machine,
			Phase:   phase,
			Error:   fmt.Errorf("unknown phase: %s", phase),
		}
	}
}

func (w *WorkflowExecutor) executeBootstrap(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement nixos-anywhere bootstrap
	return ExecutionResult{
		Machine: machine,
		Phase:   "bootstrap",
		Output:  "Bootstrap completed",
	}
}

func (w *WorkflowExecutor) executeSecrets(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement secrets deployment
	return ExecutionResult{
		Machine: machine,
		Phase:   "secrets",
		Output:  "Secrets deployed",
	}
}
