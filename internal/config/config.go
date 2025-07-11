package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Global GlobalConfig           `mapstructure:"global"`
	Flakes map[string]FlakeConfig `mapstructure:"flakes"`
}

type GlobalConfig struct {
	Tags              []string        `mapstructure:"tags"`
	RequireAllSuccess bool            `mapstructure:"requireAllSuccess"`
	AutoBootstrap     bool            `mapstructure:"autoBootstrap"`
	DisableBootstrap  bool            `mapstructure:"disableBootstrap"`
	DryRun            bool            `mapstructure:"dryRun"`
	SshClientConfig   SshClientConfig `mapstructure:"sshClientConfig"`
	Timeout           time.Duration   `mapstructure:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int             `mapstructure:"concurrency"`
	Verbose           bool            `mapstructure:"verbose"`
}

type FlakeConfig struct {
	FlakePath       string                   `mapstructure:"flakePath"`
	SshClientConfig SshClientConfig          `mapstructure:"sshClientConfig"`
	Machines        map[string]MachineConfig `mapstructure:"machines"`
}

type MachineConfig struct {
	Name                 string           `mapstructure:"name"` // Optional
	FlakeName            string           `mapstructure:"flakeName"`
	FlakePath            string           `mapstructure:"flakePath"`
	Ssh                  SshClientConfig  `mapstructure:"ssh"`
	Tags                 []string         `mapstructure:"tags"`
	FlakeOutput          string           `mapstructure:"flakeOutput"`          // Optional
	FlakeBuildOutputPath string           `mapstructure:"flakeBuildOutputPath"` // Not input
	Transport            string           `mapstructure:"transport"`            // Optional
	Errors               error            `mapstructure:"errors"`               // Not input
	Bootstrap            *BootstrapConfig `mapstructure:"bootstrap,omitempty"`
	Secrets              []SecretConfig   `mapstructure:"secrets,omitempty"`
}

type SshClientConfig struct {
	User       string `mapstructure:"user"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
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

	C.Global.Timeout *= time.Second

	if err := validateConfig(&C); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &C, nil
}

func validateConfig(c *Config) error {
	if len(c.Flakes) == 0 {
		return fmt.Errorf("flakes is required")
	}

	for flakeName, flake := range c.Flakes {
		if len(flake.Machines) == 0 {
			return fmt.Errorf("flakes[%s]machines is empty", flakeName)
		}

		for machineName, machine := range flake.Machines {
			if machine.Ssh.Host == "" {
				return fmt.Errorf("flakes[%s]machines[%s].ssh.host is empty", flakeName, machineName)
			}
		}
	}

	return nil
}

// GetMachinesByTags filters machines by tags.
func (c *Config) GetMachinesByTags(tags []string) ([]MachineConfig, error) {
	// Aggregate all machines across flakes
	var allMachines []MachineConfig
	for flakeName, flake := range c.Flakes {
		for machineName, machine := range flake.Machines {

			if machine.FlakePath == "" {
				if flake.FlakePath == "" {
					return nil, fmt.Errorf("machine.flakePath and flake.flakePath can't be empty at the same time")
				}

				machine.Name = machineName
				machine.FlakeName = flakeName
				machine.FlakePath = flake.FlakePath

				if machine.FlakeOutput == "" {
					machine.FlakeOutput = machine.Name
				}
			}

			allMachines = append(allMachines, machine)
		}
	}

	if len(tags) == 0 {
		return allMachines, nil
	}

	var filtered []MachineConfig
	for _, m := range allMachines {
		if matchesTags(m.Tags, tags) {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
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
