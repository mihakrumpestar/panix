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
	buildOutputPath string
	treeStyleParams `mapstructure:",squash"`
	Machines        map[string]*Machine `mapstructure:"machines"`
}

func (c *Configuration) SetBuildOutputPath(buildOutputPath string) {
	c.buildOutputPath = buildOutputPath
}

func (c *Configuration) GetBuildOutputPath() string {
	return c.buildOutputPath
}

type Machine struct {
	Local           bool `mapstructure:"local"`
	activationError error
	treeStyleParams `mapstructure:",squash"`
}

func (m *Machine) SetActivationError(err error) {
	m.activationError = err
}

func (m *Machine) GetActivationError() error {
	return m.activationError
}

type treeStyleParams struct {
	Ssh     *Ssh           `mapstructure:"ssh,omitempty"`
	Tags    []string       `mapstructure:"tags"`
	Secrets []SecretConfig `mapstructure:"secrets,omitempty"`
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

	C.Global.Timeout *= time.Second

	C.Flakes, err = C.filterConfigEntrys()
	if err != nil {
		panic(fmt.Errorf("failed to filter config: %w", err))
	}

	err = C.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	godump.Dump(C)

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
// An entry is kept if it matches all provided filters (flakes, configurations, machines, tags).
// If a filter type is not provided (e.g., the 'machines' slice is empty), it is not used for filtering.
func (c *Config) filterConfigEntrys() (map[string]*Flake, error) {
	cC := *c // Copy

	flakesFilter := cC.Global.Filters.Flakes
	configurationsFilter := cC.Global.Filters.Configurations
	machinesFilter := cC.Global.Filters.Machines

	for flakeName, flake := range cC.Flakes {
		if len(flakesFilter) > 0 && !slices.Contains(flakesFilter, flakeName) {
			delete(cC.Flakes, flakeName)
			continue
		}

		for configurationName, conf := range flake.Configurations {
			if len(configurationsFilter) > 0 && !slices.Contains(configurationsFilter, configurationName) {
				delete(cC.Flakes, configurationName)
				continue
			}

			for machineName, machine := range conf.Machines {
				if len(machinesFilter) > 0 && !slices.Contains(machinesFilter, machineName) {
					delete(cC.Flakes, machineName)
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
			}
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
