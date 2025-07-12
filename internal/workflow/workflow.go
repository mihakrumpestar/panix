package workflow

import (
	"context"
	"fmt"
	"slices"
	"sync"

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
		return w.executeBranching(currentPhase, nextPhases, false, false, false)
	case workflow_definition.PhaseBuild:
		return w.executeBranching(currentPhase, nextPhases, true, true, false)
	case workflow_definition.PhaseBootstrap:
		return w.executeBranching(currentPhase, nextPhases, true, true, true)
	case workflow_definition.PhaseTransfer:
		return w.executeBranching(currentPhase, nextPhases, true, true, true)
	case workflow_definition.PhaseSecrets:
		return w.executeBranching(currentPhase, nextPhases, true, true, true)
	case workflow_definition.PhaseActivate:
		return w.executeActivatePhase(nextPhases)
	case workflow_definition.PhaseStatus:
		return w.executeBranching(currentPhase, nextPhases, false, false, false)
	default:
		return nil, fmt.Errorf("unknown phase: %s", currentPhase)
	}
}

// Helpers

func (w *WorkflowExecutor) executeBranching(currentPhase workflow_definition.WorkflowPhase, nextPhases []workflow_definition.WorkflowPhase, branchFlakes, branchConfigs, branchMachines bool) (*ExecutionResult, error) {
	result := &ExecutionResult{}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var successCount int

	for flakeName, flake := range w.cfg.Flakes {
		if branchFlakes {
			wg.Add(1)
			go func(fname string, f config.Flake) {
				defer wg.Done()
				err := w.executeFlakeBranch(currentPhase, nextPhases, fname, f, branchConfigs, branchMachines)
				mu.Lock()
				if err != nil {
					errors = append(errors, fmt.Errorf("flake %s: %w", fname, err))
				} else {
					successCount++
				}
				mu.Unlock()
			}(flakeName, flake)
		} else {
			err := w.executeFlakeBranch(currentPhase, nextPhases, flakeName, flake, branchConfigs, branchMachines)
			if err != nil {
				if w.cfg.Global.RequireAllSuccess {
					return nil, fmt.Errorf("flake %s: %w", flakeName, err)
				}
				fmt.Printf("Warning: flake %s failed but continuing: %v\n", flakeName, err)
			} else {
				successCount++
			}
		}
	}

	if branchFlakes {
		wg.Wait()

		if len(errors) > 0 {
			if w.cfg.Global.RequireAllSuccess {
				return nil, fmt.Errorf("phase %s failed: %v", currentPhase, errors)
			}
			if successCount == 0 {
				return nil, fmt.Errorf("phase %s: all flakes failed", currentPhase)
			}
			for _, err := range errors {
				fmt.Printf("Warning: %v\n", err)
			}
		}
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return result, nil
}

func (w *WorkflowExecutor) executeFlakeBranch(currentPhase workflow_definition.WorkflowPhase, nextPhases []workflow_definition.WorkflowPhase, flakeName string, flake config.Flake, branchConfigs, branchMachines bool) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var successCount int

	for configName, configuration := range flake.Configurations {
		if branchConfigs {
			wg.Add(1)
			go func(cname string, conf config.Configuration) {
				defer wg.Done()
				err := w.executeConfigBranch(currentPhase, flakeName, cname, conf, branchMachines)
				mu.Lock()
				if err != nil {
					errors = append(errors, fmt.Errorf("config %s: %w", cname, err))
				} else {
					successCount++
				}
				mu.Unlock()
			}(configName, configuration)
		} else {
			err := w.executeConfigBranch(currentPhase, flakeName, configName, configuration, branchMachines)
			if err != nil {
				if w.cfg.Global.RequireAllSuccess {
					return fmt.Errorf("config %s: %w", configName, err)
				}
				fmt.Printf("Warning: config %s failed but continuing: %v\n", configName, err)
			} else {
				successCount++
			}
		}
	}

	if branchConfigs {
		wg.Wait()

		if len(errors) > 0 {
			if w.cfg.Global.RequireAllSuccess {
				return fmt.Errorf("flake %s failed: %v", flakeName, errors)
			}
			if successCount == 0 {
				return fmt.Errorf("flake %s: all configurations failed", flakeName)
			}
			for _, err := range errors {
				fmt.Printf("Warning: %v\n", err)
			}
		}
	}

	return nil
}

func (w *WorkflowExecutor) executeConfigBranch(currentPhase workflow_definition.WorkflowPhase, flakeName, configName string, configuration config.Configuration, branchMachines bool) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var successCount int

	for machineName, machine := range configuration.Machines {
		if branchMachines {
			fmt.Println("Branch machines")
			wg.Add(1)
			go func(mname string, m config.Machine) {
				defer wg.Done()
				err := w.executeMachinePhase(currentPhase, flakeName, configName, mname, m)
				mu.Lock()
				if err != nil {
					errors = append(errors, fmt.Errorf("machine %s: %w", mname, err))
				} else {
					successCount++
				}
				mu.Unlock()
			}(machineName, machine)
		} else {
			err := w.executeMachinePhase(currentPhase, flakeName, configName, machineName, machine)
			if err != nil {
				if w.cfg.Global.RequireAllSuccess {
					return fmt.Errorf("machine %s: %w", machineName, err)
				}
				fmt.Printf("Warning: machine %s failed but continuing: %v\n", machineName, err)
			} else {
				successCount++
			}
		}
	}

	if branchMachines {
		wg.Wait()

		if len(errors) > 0 {
			if w.cfg.Global.RequireAllSuccess {
				return fmt.Errorf("config %s failed: %v", configName, errors)
			}
			if successCount == 0 {
				return fmt.Errorf("config %s: all machines failed", configName)
			}
			for _, err := range errors {
				fmt.Printf("Warning: %v\n", err)
			}
		}
	}

	return nil
}

func (w *WorkflowExecutor) executeMachinePhase(currentPhase workflow_definition.WorkflowPhase, flakeName, configName, machineName string, machine config.Machine) error {
	switch currentPhase {
	case workflow_definition.PhasePreflight:
		return w.executeMachinePreflight(flakeName, configName, machineName, machine)
	case workflow_definition.PhaseBuild:
		return w.executeMachineBuild(flakeName, configName, machineName, machine)
	case workflow_definition.PhaseBootstrap:
		return w.executeMachineBootstrap(flakeName, configName, machineName, machine)
	case workflow_definition.PhaseTransfer:
		return w.executeMachineTransfer(flakeName, configName, machineName, machine)
	case workflow_definition.PhaseSecrets:
		return w.executeMachineSecrets(flakeName, configName, machineName, machine)
	case workflow_definition.PhaseStatus:
		return w.executeMachineStatus(flakeName, configName, machineName, machine)
	default:
		return fmt.Errorf("unknown phase: %s", currentPhase)
	}
}

func (w *WorkflowExecutor) executeActivatePhase(nextPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	var allMachines []machineInfo

	for flakeName, flake := range w.cfg.Flakes {
		for configName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				allMachines = append(allMachines, machineInfo{
					flakeName:   flakeName,
					configName:  configName,
					machineName: machineName,
					machine:     machine,
				})
			}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var successCount int
	var failedMachines []machineInfo

	for _, mi := range allMachines {
		wg.Add(1)
		go func(mi machineInfo) {
			defer wg.Done()
			err := w.executeMachineActivate(mi.flakeName, mi.configName, mi.machineName, mi.machine)
			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Errorf("machine %s: %w", mi.machineName, err))
				failedMachines = append(failedMachines, mi)
			} else {
				successCount++
			}
			mu.Unlock()
		}(mi)
	}

	wg.Wait()

	if len(errors) > 0 {
		if w.cfg.Global.RequireAllSuccess {
			fmt.Printf("Activation failed, rolling back all machines...\n")
			w.rollbackMachines(allMachines)
			return nil, fmt.Errorf("activation failed: %v", errors)
		}
		if successCount == 0 {
			return nil, fmt.Errorf("activation: all machines failed")
		}
		for _, err := range errors {
			fmt.Printf("Warning: %v\n", err)
		}
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}

	return &ExecutionResult{}, nil
}

type machineInfo struct {
	flakeName   string
	configName  string
	machineName string
	machine     config.Machine
}

func (w *WorkflowExecutor) rollbackMachines(machines []machineInfo) {
	var wg sync.WaitGroup
	for _, mi := range machines {
		wg.Add(1)
		go func(mi machineInfo) {
			defer wg.Done()
			err := w.executeMachineRollback(mi.flakeName, mi.configName, mi.machineName, mi.machine)
			if err != nil {
				fmt.Printf("Warning: rollback failed for machine %s: %v\n", mi.machineName, err)
			}
		}(mi)
	}
	wg.Wait()
}

func (w *WorkflowExecutor) executeMachinePreflight(flakeName, configName, machineName string, machine config.Machine) error {
	return w.preflightMachine(flakeName, configName, machineName, machine)
}

func (w *WorkflowExecutor) executeMachineBuild(flakeName, configName, machineName string, machine config.Machine) error {
	// Get the flake configuration
	flake, exists := w.cfg.Flakes[flakeName]
	if !exists {
		return fmt.Errorf("flake %s not found", flakeName)
	}

	return w.buildFlakeConfiguration(flakeName, configName, flake)
}

func (w *WorkflowExecutor) executeMachineBootstrap(flakeName, configName, machineName string, machine config.Machine) error {
	if w.cfg.Global.DryRun {
		fmt.Printf("DRY RUN: Would bootstrap %s/%s/%s\n", flakeName, configName, machineName)
		return nil
	}

	// TODO: Implement nixos-anywhere bootstrap
	if w.cfg.Global.Verbose {
		fmt.Printf("Bootstrap for machine %s/%s/%s: TODO - implement nixos-anywhere\n", flakeName, configName, machineName)
	}

	return nil
}

func (w *WorkflowExecutor) executeMachineTransfer(flakeName, configName, machineName string, machine config.Machine) error {
	// Get the build output path from metadata
	buildOutputPath := w.getBuildOutputPath(flakeName, configName)

	return w.transferToMachine(flakeName, configName, machineName, machine, buildOutputPath)
}

func (w *WorkflowExecutor) executeMachineSecrets(flakeName, configName, machineName string, machine config.Machine) error {
	if w.cfg.Global.DryRun {
		fmt.Printf("DRY RUN: Would deploy secrets to %s/%s/%s\n", flakeName, configName, machineName)
		return nil
	}

	// TODO: Implement secrets deployment
	if w.cfg.Global.Verbose {
		fmt.Printf("Secrets deployment for machine %s/%s/%s: TODO - implement secrets deployment\n", flakeName, configName, machineName)
	}

	return nil
}
func (w *WorkflowExecutor) executeMachineActivate(flakeName, configName, machineName string, machine config.Machine) error {
	// Get the build output path from metadata
	buildOutputPath := w.getBuildOutputPath(flakeName, configName)

	return w.activateMachine(flakeName, configName, machineName, machine, buildOutputPath)
}

func (w *WorkflowExecutor) executeMachineStatus(flakeName, configName, machineName string, machine config.Machine) error {
	return w.statusMachine(flakeName, configName, machineName, machine)
}

func (w *WorkflowExecutor) executeMachineRollback(flakeName, configName, machineName string, machine config.Machine) error {
	return fmt.Errorf("rollback not implemented for machine %s", machineName)
}

// Metadata management functions

func (w *WorkflowExecutor) setBuildOutputPath(flakeName, configName, buildOutputPath string) {
	// Find or create flake metadata
	var flakeMetadata *FlakeMetadata
	for i := range w.metadata.FlakesMetadata {
		if w.metadata.FlakesMetadata[i].Name == flakeName {
			flakeMetadata = &w.metadata.FlakesMetadata[i]
			break
		}
	}

	if flakeMetadata == nil {
		// Create new flake metadata
		w.metadata.FlakesMetadata = append(w.metadata.FlakesMetadata, FlakeMetadata{
			Name:            flakeName,
			ConfigsMetadata: make([]ConfigMetadata, 0),
		})
		flakeMetadata = &w.metadata.FlakesMetadata[len(w.metadata.FlakesMetadata)-1]
	}

	// Find or create config metadata
	var configMetadata *ConfigMetadata
	for i := range flakeMetadata.ConfigsMetadata {
		if flakeMetadata.ConfigsMetadata[i].Name == configName {
			configMetadata = &flakeMetadata.ConfigsMetadata[i]
			break
		}
	}

	if configMetadata == nil {
		// Create new config metadata
		flakeMetadata.ConfigsMetadata = append(flakeMetadata.ConfigsMetadata, ConfigMetadata{
			Name:             configName,
			BuildOutputPath:  buildOutputPath,
			MachinesMetadata: make([]MachineMetadata, 0),
		})
	} else {
		configMetadata.BuildOutputPath = buildOutputPath
	}
}
func (w *WorkflowExecutor) getBuildOutputPath(flakeName, configName string) string {
	// Find flake metadata
	for _, flakeMetadata := range w.metadata.FlakesMetadata {
		if flakeMetadata.Name == flakeName {
			// Find config metadata
			for _, configMetadata := range flakeMetadata.ConfigsMetadata {
				if configMetadata.Name == configName {
					return configMetadata.BuildOutputPath
				}
			}
		}
	}
	return ""
}

func (w *WorkflowExecutor) executeBootstrap(currentPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	// TODO: Implement nixos-anywhere bootstrap
	return nil, nil
}

func (w *WorkflowExecutor) executeSecrets(currentPhases []workflow_definition.WorkflowPhase) (*ExecutionResult, error) {
	// TODO: Implement secrets deployment
	return nil, nil
}
