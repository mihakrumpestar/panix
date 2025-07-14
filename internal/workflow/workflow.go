package workflow

import (
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
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
}

type WorkflowExecutorForConfigurationAndMachine struct {
	ctx context.Context
	cfg *config.Global
}

func NewWorkflowExecutor(ctx context.Context, cfg *config.Config) (*WorkflowExecutor, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.Global.Timeout)

	return &WorkflowExecutor{
		ctx:    ctx,
		cancel: cancel,
		cfg:    cfg,
	}, nil
}

// Helpers

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

			// Checker that passes only flake configurations that have at least one machine that is reachable
			atLeastOneValid := true
			for _, sm := range sms {
				if sm.FlakeName == flakeName && sm.ConfigurationName == configurationName && sm.Error != nil {
					atLeastOneValid = false
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
