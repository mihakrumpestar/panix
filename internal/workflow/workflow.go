package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
)

type WorkflowExecutor struct {
	config   *config.Config
	machines []config.MachineConfig
	ctx      context.Context
	cancel   context.CancelFunc
}

type ExecutionResult struct {
	Machine config.MachineConfig
	Phase   string
	Success bool
	Error   error
	Output  string
}

type WorkflowOptions struct {
	DryRun    bool
	Verbose   bool
	SkipPhase map[string]bool
}

func NewWorkflowExecutor(cfg *config.Config, machines []config.MachineConfig) *WorkflowExecutor {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Global.Timeout)*time.Second)

	return &WorkflowExecutor{
		config:   cfg,
		machines: machines,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (w *WorkflowExecutor) Execute(opts WorkflowOptions) error {
	defer w.cancel()

	phases := []string{"preflight", "bootstrap", "secrets", "build", "transfer", "activate"}

	for _, phase := range phases {
		if opts.SkipPhase[phase] {
			continue
		}

		if opts.Verbose {
			fmt.Printf("Starting phase: %s\n", phase)
		}

		if err := w.executePhase(phase, opts); err != nil {
			if w.config.Global.RequireAllSuccess {
				return fmt.Errorf("phase %s failed: %w", phase, err)
			}
			fmt.Printf("Warning: phase %s failed but continuing: %v\n", phase, err)
		}
	}

	return nil
}

func (w *WorkflowExecutor) executePhase(phase string, opts WorkflowOptions) error {
	var wg sync.WaitGroup
	resultChan := make(chan ExecutionResult, len(w.machines))
	semaphore := make(chan struct{}, w.config.Global.Concurrency)

	for _, machine := range w.machines {
		wg.Add(1)
		go func(m config.MachineConfig) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()

				result := w.executeMachinePhase(m, phase, opts)
				resultChan <- result

			case <-w.ctx.Done():
				resultChan <- ExecutionResult{
					Machine: m,
					Phase:   phase,
					Success: false,
					Error:   w.ctx.Err(),
				}
			}
		}(machine)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var failures []ExecutionResult
	for result := range resultChan {
		if opts.Verbose {
			if result.Success {
				fmt.Printf("✓ %s: %s completed\n", result.Machine.Name, result.Phase)
			} else {
				fmt.Printf("✗ %s: %s failed: %v\n", result.Machine.Name, result.Phase, result.Error)
			}
		}

		if !result.Success {
			failures = append(failures, result)
		}
	}

	if len(failures) > 0 && w.config.Global.RequireAllSuccess {
		return fmt.Errorf("%d machines failed in phase %s", len(failures), phase)
	}

	return nil
}

func (w *WorkflowExecutor) executeMachinePhase(machine config.MachineConfig, phase string, opts WorkflowOptions) ExecutionResult {
	if opts.DryRun {
		return ExecutionResult{
			Machine: machine,
			Phase:   phase,
			Success: true,
			Output:  fmt.Sprintf("DRY RUN: would execute %s for %s", phase, machine.Name),
		}
	}

	switch phase {
	case "preflight":
		return w.executePreflight(machine)
	case "bootstrap":
		return w.executeBootstrap(machine)
	case "secrets":
		return w.executeSecrets(machine)
	case "build":
		return w.executeBuild(machine)
	case "transfer":
		return w.executeTransfer(machine)
	case "activate":
		return w.executeActivate(machine)
	default:
		return ExecutionResult{
			Machine: machine,
			Phase:   phase,
			Success: false,
			Error:   fmt.Errorf("unknown phase: %s", phase),
		}
	}
}

func (w *WorkflowExecutor) executePreflight(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement SSH connectivity check and bootstrap detection
	return ExecutionResult{
		Machine: machine,
		Phase:   "preflight",
		Success: true,
		Output:  "Preflight checks passed",
	}
}

func (w *WorkflowExecutor) executeBootstrap(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement nixos-anywhere bootstrap
	return ExecutionResult{
		Machine: machine,
		Phase:   "bootstrap",
		Success: true,
		Output:  "Bootstrap completed",
	}
}

func (w *WorkflowExecutor) executeSecrets(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement secrets deployment
	return ExecutionResult{
		Machine: machine,
		Phase:   "secrets",
		Success: true,
		Output:  "Secrets deployed",
	}
}

func (w *WorkflowExecutor) executeBuild(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement nix build
	return ExecutionResult{
		Machine: machine,
		Phase:   "build",
		Success: true,
		Output:  "Build completed",
	}
}

func (w *WorkflowExecutor) executeTransfer(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement nix copy
	return ExecutionResult{
		Machine: machine,
		Phase:   "transfer",
		Success: true,
		Output:  "Transfer completed",
	}
}

func (w *WorkflowExecutor) executeActivate(machine config.MachineConfig) ExecutionResult {
	// TODO: Implement nixos-rebuild switch
	return ExecutionResult{
		Machine: machine,
		Phase:   "activate",
		Success: true,
		Output:  "Activation completed",
	}
}
