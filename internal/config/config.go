package config

import (
	"fmt"
	"slices"
	"time"

	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/spf13/viper"
	"github.com/yassinebenaid/godump"
)

type Config struct {
	Global Global            `mapstructure:"global"`
	Flakes map[string]*Flake `mapstructure:"flakes"`
}

type Global struct {
	Filters           Filters                             `mapstructure:"filters"`
	RequireAllSuccess bool                                `mapstructure:"requireAllSuccess"`
	AutoBootstrap     bool                                `mapstructure:"autoBootstrap"`
	DryRun            bool                                `mapstructure:"dryRun"`
	Ssh               *Ssh                                `mapstructure:"ssh"`
	Timeout           time.Duration                       `mapstructure:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int                                 `mapstructure:"concurrency"`
	SkipPhases        []workflow_definition.WorkflowPhase `mapstructure:"skipPhases"`
	Verbose           bool                                `mapstructure:"verbose"`
	Debug             bool                                `mapstructure:"debug"`
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
	Configurations  map[string]*Configuration `mapstructure:"configurations"`
}

type Configuration struct {
	FlakeOutput     string `mapstructure:"flakeOutput"` // Override if not standard style
	treeStyleParams `mapstructure:",squash"`
	Machines        map[string]*Machine `mapstructure:"machines"`
}

type Machine struct {
	Local           bool `mapstructure:"local"`
	treeStyleParams `mapstructure:",squash"`
}

type treeStyleParams struct {
	Ssh      *Ssh           `mapstructure:"ssh,omitempty"`
	Tags     []string       `mapstructure:"tags"`
	Secrets  []SecretConfig `mapstructure:"secrets,omitempty"`
	Disabled bool           `mapstructure:"disabled"`
}

type Ssh struct {
	Alias      string `mapstructure:"alias"`
	User       string `mapstructure:"user"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	PrivateKey string `mapstructure:"privateKey"`
	PublicKey  string `mapstructure:"publicKey"`
}

type SecretConfig struct {
	LocalPath  string `mapstructure:"localPath"`
	RemotePath string `mapstructure:"remotePath"`
	Mode       string `mapstructure:"mode"`
}

var (
	ConfigFile string
	C          Config
)

// LoadConfig reads and parses the config file.
func LoadConfig() (*Config, error) {
	vpr := viper.New()

	// File config
	vpr.SetConfigFile(ConfigFile)
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

	// Defaults
	C.Global.Timeout *= time.Second

	C.Flakes, err = C.filterConfigEntrys()
	if err != nil {
		panic(fmt.Errorf("failed to filter config: %w", err))
	}

	err = C.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if C.Global.Debug {
		godump.Dump(C)
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

// FilterConfigEntrys filters the configuration based on command-line or global selections.
// An entry is kept if it matches all provided filters (flakes, configurations, machines, tags) and is not disabled.
// If a filter type is not provided (e.g., the 'machines' slice is empty), it is not used for filtering.
func (c *Config) filterConfigEntrys() (map[string]*Flake, error) {
	cC := *c // Copy

	flakesFilter := cC.Global.Filters.Flakes
	configurationsFilter := cC.Global.Filters.Configurations
	machinesFilter := cC.Global.Filters.Machines

	for flakeName, flake := range cC.Flakes {
		if (len(flakesFilter) > 0 && !slices.Contains(flakesFilter, flakeName)) || flake.Disabled {
			delete(cC.Flakes, flakeName)
			continue
		}

		for configurationName, configuration := range flake.Configurations {
			if (len(configurationsFilter) > 0 && !slices.Contains(configurationsFilter, configurationName)) || configuration.Disabled {
				delete(flake.Configurations, configurationName)
				continue
			}

			for machineName, machine := range configuration.Machines {
				if (len(machinesFilter) > 0 && !slices.Contains(machinesFilter, machineName)) || flake.Disabled {
					delete(configuration.Machines, machineName)
					continue
				}

				// A machine must match the tag filters if they are provided.
				if len(cC.Global.Filters.Tags) > 0 {
					allMachineTags := flake.Tags
					allMachineTags = append(allMachineTags, configuration.Tags...)
					allMachineTags = append(allMachineTags, machine.Tags...)

					if !matchesTags(allMachineTags, C.Global.Filters.Tags) {
						delete(configuration.Machines, machineName)
						continue
					}
				}
			}

			if len(configuration.Machines) == 0 {
				delete(flake.Configurations, configurationName)
			}
		}

		if len(flake.Configurations) == 0 {
			delete(cC.Flakes, flakeName)
		}
	}

	if len(cC.Flakes) == 0 {
		return nil, fmt.Errorf("flakes configuration empty after filtering")
	}

	return cC.Flakes, nil
}

func matchesTags(tags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	for _, filterTag := range filterTags {
		if !slices.Contains(tags, filterTag) {
			return false
		}
	}

	return true
}
