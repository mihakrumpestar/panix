package config

import (
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/spf13/viper"
)

type Config struct {
	Global Global           `mapstructure:"global"`
	Flakes map[string]Flake `mapstructure:"flakes"`
}

type Global struct {
	Filters           Filters                             `mapstructure:"filters"`
	RequireAllSuccess bool                                `mapstructure:"requireAllSuccess"`
	AutoBootstrap     bool                                `mapstructure:"autoBootstrap"`
	DryRun            bool                                `mapstructure:"dryRun"`
	Ssh               Ssh                                 `mapstructure:"ssh"`
	Timeout           time.Duration                       `mapstructure:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int                                 `mapstructure:"concurrency"`
	SkipPhases        []workflow_definition.WorkflowPhase `mapstructure:"skipPhases"`
	Verbose           bool                                `mapstructure:"verbose"`
}

type Filters struct {
	Flakes         []string `mapstructure:"flakes"`
	Configurations []string `mapstructure:"configurations"`
	Machines       []string `mapstructure:"machines"`
	Tags           []string `mapstructure:"tags"`
}

type Flake struct {
	Url             string `mapstructure:"url"` // Flake path or url
	treeStyleParams `mapstructure:",squash"`
	Configurations  map[string]Configuration `mapstructure:"configurations"`
}

type Configuration struct {
	FlakeOutput     string `mapstructure:"flakeOutput"` // Override if not standard style
	treeStyleParams `mapstructure:",squash"`
	Machines        map[string]Machine `mapstructure:"machines"`
}

type Machine struct {
	Local           bool `mapstructure:"local"`
	treeStyleParams `mapstructure:",squash"`
}

type treeStyleParams struct {
	Ssh       Ssh              `mapstructure:"ssh"`
	Tags      []string         `mapstructure:"tags"`
	Bootstrap *BootstrapConfig `mapstructure:"bootstrap,omitempty"`
	Secrets   []SecretConfig   `mapstructure:"secrets,omitempty"`
}

type Ssh struct {
	Alias      string `mapstructure:"alias"`
	User       string `mapstructure:"user"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port,omitzero"`
	PrivateKey string `mapstructure:"privateKey"`
	PublicKey  string `mapstructure:"publicKey"`
}

type BootstrapConfig struct {
	FlakeAttr string            `mapstructure:"flakeAttr"`
	ExtraArgs []string          `mapstructure:"extraArgs"`
	Env       map[string]string `mapstructure:"env"`
}

type SecretConfig struct {
	LocalPath  string `mapstructure:"localPath"`
	RemotePath string `mapstructure:"remotePath"`
	Mode       string `mapstructure:"mode"`
}

var C Config

// LoadConfig reads and parses the config file.
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "panix.yml"
	}

	vpr := viper.New()

	// File config
	vpr.SetConfigFile(configPath)
	// ENV config
	vpr.SetEnvPrefix("PANIX")

	err := vpr.ReadInConfig() // Find and read the config file
	if err != nil {           // Handle errors reading the config file
		return nil, fmt.Errorf("fatal error config file: %w", err)
	}

	err = vpr.UnmarshalExact(&C)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}

	C.Global.Timeout *= time.Second

	err = C.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	C.Flakes, err = C.filterConfigEntrys()
	if err != nil {
		panic(fmt.Errorf("failed to filter config: %w", err))
	}

	return &C, nil
}

func (c *Config) validateConfig() error {
	if len(c.Flakes) == 0 {
		return fmt.Errorf("flakes is required")
	}

	for flakeName, flake := range c.Flakes {
		if len(flake.Configurations) == 0 {
			return fmt.Errorf("flakes[%s]configurations is empty", flakeName)
		}

		for configurationName, configuration := range flake.Configurations {
			if len(configuration.Machines) == 0 {
				return fmt.Errorf("flakes[%s]configurations[%s]machines is empty", flakeName, configurationName)
			}

			for machineName, machine := range configuration.Machines {
				if !machine.Local && machine.Ssh.Alias == "" && machine.Ssh.Host == "" {
					return fmt.Errorf("flakes[%s]configurations[%s]machines[%s].ssh is not configured and deploy is not local", flakeName, configurationName, machineName)
				}
			}
		}
	}

	return nil
}

// FilterConfigEntrys filters the configuration based on command-line selections.
// An entry is kept if it matches all provided filters (flakes, configurations, machines, tags).
// If a filter type is not provided (e.g., the 'machines' slice is empty), it is not used for filtering.
func (c *Config) filterConfigEntrys() (map[string]Flake, error) {
	filteredFlakes := make(map[string]Flake)

	toSet := func(s []string) map[string]bool {
		set := make(map[string]bool, len(s))
		for _, item := range s {
			set[item] = true
		}
		return set
	}

	flakesSet := toSet(C.Global.Filters.Flakes)
	configurationsSet := toSet(C.Global.Filters.Configurations)
	machinesSet := toSet(C.Global.Filters.Machines)

	for flakeName, flake := range c.Flakes {
		if len(flakesSet) > 0 && !flakesSet[flakeName] {
			continue
		}

		filteredConfs := make(map[string]Configuration)
		for confName, conf := range flake.Configurations {
			if len(configurationsSet) > 0 && !configurationsSet[confName] {
				continue
			}

			filteredMachines := make(map[string]Machine)
			for machineName, machine := range conf.Machines {
				if len(machinesSet) > 0 && !machinesSet[machineName] {
					continue
				}

				// A machine must match the tag filters if they are provided.
				if len(C.Global.Filters.Tags) > 0 {
					allMachineTags := append([]string{}, flake.Tags...)
					allMachineTags = append(allMachineTags, conf.Tags...)
					allMachineTags = append(allMachineTags, machine.Tags...)

					if !matchesTags(allMachineTags, C.Global.Filters.Tags) {
						continue // Skip this machine if tags don't match
					}
				}

				// If we reach here, the machine has passed all filters.
				filteredMachines[machineName] = machine
			}

			if len(filteredMachines) > 0 {
				newConf := conf
				newConf.Machines = filteredMachines
				filteredConfs[confName] = newConf
			}
		}

		if len(filteredConfs) > 0 {
			newFlake := flake
			newFlake.Configurations = filteredConfs
			filteredFlakes[flakeName] = newFlake
		}
	}

	if len(filteredFlakes) == 0 {
		return nil, fmt.Errorf("flakes configuration empty after filtering")
	}

	return filteredFlakes, nil
}

func matchesTags(machineTags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	tagSet := make(map[string]bool)

	for _, t := range machineTags {
		tagSet[t] = true
	}

	for _, ft := range filterTags {
		switch ft[0] {
		case '+':
			if !tagSet[ft[1:]] {
				return false
			}
		case '-':
			if tagSet[ft[1:]] {
				return false
			}
		default:
			if !tagSet[ft] {
				return false
			}
		}
	}

	return true
}
