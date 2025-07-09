package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Global GlobalConfig  `mapstructure:"global"`
	Flakes []FlakeConfig `mapstructure:"flakes"`
}

type GlobalConfig struct {
	Tags              []string `mapstructure:"tags"`
	RequireAllSuccess bool     `mapstructure:"requireAllSuccess"`
	AutoBootstrap     bool     `mapstructure:"autoBootstrap"`
	DisableBootstrap  bool     `mapstructure:"disableBootstrap"`
	DryRun            bool     `mapstructure:"dryRun"`
	SSHUser           string   `mapstructure:"sshUser"`
	SSHPrivateKey     string   `mapstructure:"sshPrivateKey"`
	Timeout           int      `mapstructure:"timeout"`
	Concurrency       int      `mapstructure:"concurrency"`
	Verbose           bool     `mapstructure:"verbose"`
}

type FlakeConfig struct {
	Flake    string          `mapstructure:"flake"`
	Machines []MachineConfig `mapstructure:"machines"`
}

type MachineConfig struct {
	Name        string           `mapstructure:"name"`
	Host        string           `mapstructure:"host"`
	User        string           `mapstructure:"user"`
	Port        int              `mapstructure:"port"`
	Tags        []string         `mapstructure:"tags"`
	FlakeOutput string           `mapstructure:"flakeOutput"`
	Bootstrap   *BootstrapConfig `mapstructure:"bootstrap,omitempty"`
	Secrets     []SecretConfig   `mapstructure:"secrets,omitempty"`
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

	// Defaults (already specified in cmd.root at flag init)
	//viper.SetDefault("global.concurrency", 4)
	//viper.SetDefault("global.timeout", 300)

	// File config
	vpr.SetConfigFile(configPath)
	// ENV config
	vpr.SetEnvPrefix("PANIX")

	err := vpr.ReadInConfig() // Find and read the config file
	if err != nil {           // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	err = vpr.Unmarshal(&C)
	if err != nil {
		panic(fmt.Errorf("unable to decode into struct, %v", err))
	}

	if err := validateConfig(&C); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &C, nil
}

func validateConfig(c *Config) error {
	if len(c.Flakes) == 0 {
		return fmt.Errorf("flakes is required")
	}

	for i, flake := range c.Flakes {

		if flake.Flake == "" {
			return fmt.Errorf("flakes[%d].flake path is required", i)
		}

		if len(flake.Machines) == 0 {
			return fmt.Errorf("at least one machine must be configured")
		}

		for i, machine := range flake.Machines {
			if machine.Name == "" {
				return fmt.Errorf("machine[%d].name is required", i)
			}

			if machine.Host == "" {
				return fmt.Errorf("machine[%d].host is required", i)
			}

			if machine.FlakeOutput == "" {
				return fmt.Errorf("machine[%d].flakeOutput is required", i)
			}
		}
	}

	return nil
}

// GetMachinesByTags filters machines by tags.
func (c *Config) GetMachinesByTags(tags []string) []MachineConfig {
	// Aggregate all machines across flakes
	var allMachines []MachineConfig
	for _, flake := range c.Flakes {
		allMachines = append(allMachines, flake.Machines...)
	}
	if len(tags) == 0 {
		return allMachines
	}
	var filtered []MachineConfig
	for _, m := range allMachines {
		if matchesTags(m.Tags, tags) {
			filtered = append(filtered, m)
		}
	}
	return filtered
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
