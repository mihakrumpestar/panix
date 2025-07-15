package workflow

import (
	"context"
	"net/url"

	"github.com/mihakrumpestar/panix/internal/config"
)

type MetadataID struct {
	FlakeName         string
	ConfigurationName string
	MachineName       *url.URL
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

func forAllMachines(conf map[string]*config.Flake, function func(i int, flakeName, configurationName string, machineName *url.URL, machine *config.Machine)) {
	i := 0
	for flakeName, flake := range conf {
		for configurationName, configuration := range flake.Configurations {
			for machineName, machine := range configuration.Machines {
				function(i, flakeName, configurationName, &machineName, machine)
				i++
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
