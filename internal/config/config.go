package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Global   GlobalConfig    `toml:"global"`
	Machines []MachineConfig `toml:"machine"`
}

type GlobalConfig struct {
	Flake             string `yaml:"flake"`
	RequireAllSuccess bool   `yaml:"requireAllSuccess"`
	AutoBootstrap     bool   `yaml:"autoBootstrap"`
	Concurrency       int    `yaml:"concurrency"`
	Timeout           int    `yaml:"timeout"`
}

type MachineConfig struct {
	Name        string           `toml:"name"`
	Host        string           `toml:"host"`
	User        string           `toml:"user"`
	Port        int              `toml:"port"`
	Tags        []string         `toml:"tags"`
	FlakeOutput string           `toml:"flakeOutput"`
	Bootstrap   *BootstrapConfig `toml:"bootstrap,omitempty"`
	Secrets     []SecretConfig   `toml:"secrets,omitempty"`
}

type BootstrapConfig struct {
	FlakeAttr string            `toml:"flakeAttr"`
	ExtraArgs []string          `toml:"extraArgs"`
	Env       map[string]string `toml:"env"`
}

type SecretConfig struct {
	LocalPath  string `yaml:"localPath"`
	RemotePath string `yaml:"remotePath"`
	Mode       string `yaml:"mode"`
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "panix.toml"
	}

	if !filepath.IsAbs(configPath) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		configPath = filepath.Join(wd, configPath)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	var config Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	setDefaults(&config)

	return &config, nil
}

func validateConfig(config *Config) error {
	if config.Global.Flake == "" {
		return fmt.Errorf("global.flake is required")
	}

	if len(config.Machines) == 0 {
		return fmt.Errorf("at least one machine must be configured")
	}

	for i, machine := range config.Machines {
		if machine.Name == "" {
			return fmt.Errorf("machine[%d].name is required", i)
		}
		if machine.Host == "" {
			return fmt.Errorf("machine[%d].host is required", i)
		}
		if machine.User == "" {
			return fmt.Errorf("machine[%d].user is required", i)
		}
		if machine.FlakeOutput == "" {
			return fmt.Errorf("machine[%d].flakeOutput is required", i)
		}
	}

	return nil
}

func setDefaults(config *Config) {
	if config.Global.Concurrency == 0 {
		config.Global.Concurrency = 4
	}
	if config.Global.Timeout == 0 {
		config.Global.Timeout = 300
	}

	for i := range config.Machines {
		if config.Machines[i].Port == 0 {
			config.Machines[i].Port = 22
		}
	}
}

func (c *Config) GetMachinesByTags(tags []string) []MachineConfig {
	if len(tags) == 0 {
		return c.Machines
	}

	var filtered []MachineConfig
	for _, machine := range c.Machines {
		if matchesTags(machine.Tags, tags) {
			filtered = append(filtered, machine)
		}
	}
	return filtered
}

func matchesTags(machineTags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	machineTagSet := make(map[string]bool)
	for _, tag := range machineTags {
		machineTagSet[tag] = true
	}

	for _, filterTag := range filterTags {
		if filterTag[0] == '+' {
			if !machineTagSet[filterTag[1:]] {
				return false
			}
		} else if filterTag[0] == '-' {
			if machineTagSet[filterTag[1:]] {
				return false
			}
		} else {
			if !machineTagSet[filterTag] {
				return false
			}
		}
	}

	return true
}
