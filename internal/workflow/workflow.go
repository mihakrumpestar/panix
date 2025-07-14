package workflow

import (
	"context"
	"fmt"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

type MetadataID struct {
	FlakeName         string
	ConfigurationName string
	MachineName       string
}

type ActivationMetadata struct {
	Error error
}

type WorkflowExecutor struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	phases []workflow_definition.WorkflowPhase
}

type WorkflowExecutorConfigurationMachine struct {
	ctx context.Context
	cfg *config.Global
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
		return nil, fmt.Errorf("no phases left after filtering")
	}

	return &WorkflowExecutor{
		ctx:    ctx,
		cancel: cancel,
		phases: filteredPhases,
		cfg:    cfg,
	}, nil
}

// Execute runs the workflow phases
func (w *WorkflowExecutor) Execute() error {
	defer w.cancel()
	return w.executePhase(w.phases)
}

func (w *WorkflowExecutor) executePhase(currentPhases []workflow_definition.WorkflowPhase) error {
	if len(currentPhases) == 0 {
		return nil
	}

	currentPhase, nextPhases := currentPhases[0], currentPhases[1:]
	if w.cfg.Global.Verbose {
		fmt.Printf("starting phase: %s\n", currentPhase)
	}

	switch currentPhase {
	case workflow_definition.PhaseStatus:
		_, err := w.executeStatusPhase(nextPhases)
		return err
	//case workflow_definition.PhaseBuild:
	//	return w.executeBuildPhase(nextPhases)
	case workflow_definition.PhaseBootstrap:
		return w.executeBootstrapPhase(nextPhases)
	case workflow_definition.PhaseTransfer:
		return w.executeTransferPhase(nextPhases)
	case workflow_definition.PhaseSecrets:
		return w.executeSecretsPhase(nextPhases)
	//case workflow_definition.PhaseActivate:
	//	return w.executeActivatePhase(nextPhases)
	case workflow_definition.PhaseRollback:
		return w.executeRollbackPhase(nextPhases)
	default:
		return fmt.Errorf("unknown phase: %s", currentPhase)
	}
}

/*
// executeBuildPhase runs builds in parallel across configurations
// As soon as a configuration succeeds, applicable machines proceed with bootstrap/transfer/secrets
func (w *WorkflowExecutor) executeBuildPhase(nextPhases []workflow_definition.WorkflowPhase) error {
	if w.cfg.Global.Verbose {
		fmt.Println("Executing build phase across configurations")
	}

	// Create a pool for build tasks
	buildPool, err := ants.NewPool(w.cfg.Global.Concurrency)
	if err != nil {
		return fmt.Errorf("failed to create build worker pool: %w", err)
	}
	defer buildPool.Release()

	// Create a pool for post-build machine tasks
	machinePool, err := ants.NewPool(w.cfg.Global.Concurrency)
	if err != nil {
		return fmt.Errorf("failed to create machine worker pool: %w", err)
	}
	defer machinePool.Release()

	var wg sync.WaitGroup
	var buildErrors []error
	var buildMu sync.Mutex
	var machineErrors []error
	var machineMu sync.Mutex

	// Track which configurations have been built successfully
	successfulConfigs := make(map[string]bool)
	var configMu sync.Mutex

	// Channel to signal when a configuration is built
	configReady := make(chan struct {
		flakeName   string
		configName  string
		buildOutput string
	}, 100)

	// Submit all build tasks
	for flakeName, flake := range w.cfg.Flakes {
		for configName := range flake.Configurations {
			wg.Add(1)
			f, c := flakeName, configName
			err := buildPool.Submit(func() {
				defer wg.Done()

				if err := w.buildFlakeConfiguration(f, c, flake); err != nil {
					buildMu.Lock()
					buildErrors = append(buildErrors, fmt.Errorf("build failed for %s/%s: %w", f, c, err))
					buildMu.Unlock()
					return
				}

				buildOutput := w.cfg.Flakes[f].Configurations[c].Metadata.BuildOutputPath

				configMu.Lock()
				successfulConfigs[f+"/"+c] = true
				configMu.Unlock()

				configReady <- struct {
					flakeName   string
					configName  string
					buildOutput string
				}{f, c, buildOutput}
			})
			if err != nil {
				wg.Done()
				return fmt.Errorf("failed to submit build task: %w", err)
			}
		}
	}

	// Start a goroutine to handle post-build machine tasks
	go func() {
		for config := range configReady {
			// Find all machines for this configuration
			flake := w.cfg.Flakes[config.flakeName]
			configuration := flake.Configurations[config.configName]

			for machineName, machine := range configuration.Machines {
				if machine.Disabled {
					continue
				}

				wg.Add(1)
				f, c, m, mach := config.flakeName, config.configName, machineName, machine
				err := machinePool.Submit(func() {
					defer wg.Done()

					// Execute bootstrap, transfer, and secrets in sequence
					if err := w.executeMachineBootstrap(f, c, m, mach); err != nil {
						machineMu.Lock()
						machineErrors = append(machineErrors, fmt.Errorf("bootstrap failed for %s/%s/%s: %w", f, c, m, err))
						machineMu.Unlock()
						return
					}

					if err := w.transferToMachine(f, c, m, mach); err != nil {
						machineMu.Lock()
						machineErrors = append(machineErrors, fmt.Errorf("transfer failed for %s/%s/%s: %w", f, c, m, err))
						machineMu.Unlock()
						return
					}

					if err := w.executeMachineSecrets(f, c, m, mach); err != nil {
						machineMu.Lock()
						machineErrors = append(machineErrors, fmt.Errorf("secrets failed for %s/%s/%s: %w", f, c, m, err))
						machineMu.Unlock()
						return
					}
				})
				if err != nil {
					wg.Done()
					machineMu.Lock()
					machineErrors = append(machineErrors, fmt.Errorf("failed to submit machine task: %w", err))
					machineMu.Unlock()
				}
			}
		}
	}()

	wg.Wait()
	close(configReady)

	// Check for errors
	if len(buildErrors) > 0 && w.cfg.Global.RequireAllSuccess {
		return fmt.Errorf("build phase failed: %v", buildErrors)
	}

	if len(machineErrors) > 0 && w.cfg.Global.RequireAllSuccess {
		return fmt.Errorf("machine phases failed: %v", machineErrors)
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}

// executeActivatePhase requires all previous parallel phases to complete for all machines
func (w *WorkflowExecutor) executeActivatePhase(nextPhases []workflow_definition.WorkflowPhase) error {
	if w.cfg.Global.Verbose {
		fmt.Println("Executing activate phase - waiting for all machines to be ready")
	}

	// Create a pool for activation tasks
	pool, err := ants.NewPool(w.cfg.Global.Concurrency)
	if err != nil {
		return fmt.Errorf("failed to create activation worker pool: %w", err)
	}
	defer pool.Release()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error
	var successfulActivations []string

	// Collect all machines
	machines := w.collectAllMachines()

	for _, machine := range machines {
		if machine.machine.Disabled {
			continue
		}

		wg.Add(1)
		m := machine
		err := pool.Submit(func() {
			defer wg.Done()

			if err := w.activateMachine(m.flakeName, m.configName, m.machineName, m.machine); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("activation failed for %s/%s/%s: %w", m.flakeName, m.configName, m.machineName, err))
				mu.Unlock()
			} else {
				mu.Lock()
				successfulActivations = append(successfulActivations, fmt.Sprintf("%s/%s/%s", m.flakeName, m.configName, m.machineName))
				mu.Unlock()
			}
		})
		if err != nil {
			wg.Done()
			return fmt.Errorf("failed to submit activation task: %w", err)
		}
	}

	wg.Wait()

	// Handle rollback if required
	if len(errors) > 0 && w.cfg.Global.RequireAllSuccess {
		fmt.Printf("Activation failed, rolling back successful activations: %v\n", errors)

		// Rollback successful activations
		rollbackPool, err := ants.NewPool(w.cfg.Global.Concurrency)
		if err != nil {
			return fmt.Errorf("failed to create rollback worker pool: %w", err)
		}
		defer rollbackPool.Release()

		var rollbackWg sync.WaitGroup
		var rollbackMu sync.Mutex
		var rollbackErrors []error

		for _, activation := range successfulActivations {
			rollbackWg.Add(1)
			activationStr := activation
			err := rollbackPool.Submit(func() {
				defer rollbackWg.Done()

				// Parse activation string
				parts := strings.Split(activationStr, "/")
				if len(parts) != 3 {
					rollbackMu.Lock()
					rollbackErrors = append(rollbackErrors, fmt.Errorf("invalid activation format: %s", activationStr))
					rollbackMu.Unlock()
					return
				}

				flakeName, configName, machineName := parts[0], parts[1], parts[2]
				machine := w.cfg.Flakes[flakeName].Configurations[configName].Machines[machineName]

				if err := w.executeMachineRollback(flakeName, configName, machineName, machine); err != nil {
					rollbackMu.Lock()
					rollbackErrors = append(rollbackErrors, fmt.Errorf("rollback failed for %s: %w", activationStr, err))
					rollbackMu.Unlock()
				}
			})
			if err != nil {
				rollbackWg.Done()
				rollbackMu.Lock()
				rollbackErrors = append(rollbackErrors, fmt.Errorf("failed to submit rollback task: %w", err))
				rollbackMu.Unlock()
			}
		}

		rollbackWg.Wait()

		if len(rollbackErrors) > 0 {
			return fmt.Errorf("activation failed and rollback encountered errors: %v (original errors: %v)", rollbackErrors, errors)
		}

		return fmt.Errorf("activation failed, rollback completed: %v", errors)
	}

	if len(errors) > 0 && !w.cfg.Global.RequireAllSuccess {
		for _, err := range errors {
			fmt.Printf("Warning: %v\n", err)
		}
	}

	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}
*/

// Helper functions for other phases
func (w *WorkflowExecutor) executeBootstrapPhase(nextPhases []workflow_definition.WorkflowPhase) error {
	// Bootstrap is now handled within build phase
	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}

func (w *WorkflowExecutor) executeTransferPhase(nextPhases []workflow_definition.WorkflowPhase) error {
	// Transfer is now handled within build phase
	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}

func (w *WorkflowExecutor) executeSecretsPhase(nextPhases []workflow_definition.WorkflowPhase) error {
	// Secrets is now handled within build phase
	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}

func (w *WorkflowExecutor) executeRollbackPhase(nextPhases []workflow_definition.WorkflowPhase) error {
	// Rollback is handled within activate phase
	if len(nextPhases) > 0 {
		return w.executePhase(nextPhases)
	}
	return nil
}

func forAllMachines(conf map[string]*config.Flake, function func(flakeName, configurationName, machineName string, machine *config.Machine)) {
	for flakeName, flake := range conf {
		for configurationName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				function(flakeName, configurationName, machineName, machine)
			}
		}
	}
}

func forAllConfigurations(conf map[string]*config.Flake, sms []StatusMetadata, function func(flakeName, configurationName string, flake *config.Flake, configuration *config.Configuration)) {
	for flakeName, flake := range conf {
		for configurationName, configuration := range flake.Configurations {
			atLeastOneValid := false
			for _, sm := range sms {
				if sm.FlakeName == flakeName && sm.ConfigurationName == configurationName && sm.Error == nil {
					atLeastOneValid = true
					continue
				}
			}

			if !atLeastOneValid {
				continue
			}

			function(flakeName, configurationName, flake, configuration)
		}
	}
}
