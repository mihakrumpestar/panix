package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/gobeam/stringy"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// LoadConfig reads and parses the config file.
func LoadConfig(configFile string, flags *pflag.FlagSet) (*Config, error) {
	k := koanf.New(".")

	// Load YAML config file
	err := k.Load(file.Provider(configFile), yaml.Parser())
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %w", err)
	}

	// Load environment variables with PANIX_ prefix
	err = k.Load(
		env.Provider(
			"PANIX_",
			".",
			func(s string) string {
				// Convert PANIX_SOME_KEY to some.key
				return strings.ToLower(strings.TrimPrefix(s, "PANIX_"))
			},
		),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Load command line flags if provided
	if flags != nil {
		err := k.Load(
			posflag.ProviderWithFlag(
				flags,
				".",
				k,
				func(f *pflag.Flag) (string, any) {
					// Transform the key in whatever manner.
					keyRaw := stringy.New(f.Name).CamelCase()
					// If special nested "filters"
					if slices.Contains([]string{"flakes", "configurations", "machines"}, keyRaw.Get()) {
						keyRaw.Prefix("filters.")
					}

					key := keyRaw.Prefix("global.")

					val := posflag.FlagVal(flags, f)

					return key, val
				},
			),
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("error loading command line flags: %w", err)
		}
	}

	// First, unmarshal into conf structure with regular maps
	conf := &Config{}
	err = k.UnmarshalWithConf("", &conf, koanf.UnmarshalConf{
		Tag: "yaml",
		DecoderConfig: &mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           &conf,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	// Convert timeout from miliseconds to seconds for duration
	conf.Global.Timeout *= time.Second

	err = conf.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	err = conf.filterAndExpandConfigEntrys()
	if err != nil {
		return nil, fmt.Errorf("failed to filter config: %w", err)
	}

	err = conf.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Config won't be changing after this point

	return conf, nil
}

func (c *Config) validateConfig() error {
	if c.Flakes == nil {
		return fmt.Errorf("flakes is nil")
	}
	if len(c.Flakes) == 0 {
		return fmt.Errorf("flakes is required")
	}

	for flakeName, flake := range c.Flakes.Range(true) {
		if flake.Url == "" {
			return fmt.Errorf("flake %s has no URL configured", flakeName)
		}

		if flake.Configurations == nil || len(flake.Configurations) == 0 {
			return fmt.Errorf("flakes[%s]configurations is empty", flakeName)
		}

		for configurationName, configuration := range flake.Configurations.Range(true) {
			if configuration.Machines == nil || len(configuration.Machines) == 0 {
				return fmt.Errorf("flakes[%s]configurations[%s]machines is empty", flakeName, configurationName)
			}

			for machineName, _ := range configuration.Machines.Range(true) {
				parsedMachineName, err := url.Parse("ssh://" + machineName)
				if err != nil {
					return fmt.Errorf("flakes[%s]configurations[%s]machines[%s] has invalid machine name, it has to be formatted as URL: %w", flakeName, configurationName, machineName, err)
				}

				if parsedMachineName.Host == "" {
					return fmt.Errorf("flakes[%s]configurations[%s]machines[%s] has empty parsed host field", flakeName, configurationName, machineName)
				}
			}
		}
	}

	return nil
}

// FilterConfigEntrys filters the configuration based on command-line or global selections.
// An entry is kept if it matches all provided filters (flakes, configurations, machines, tags) and is not disabled.
// If a filter type is not provided (e.g., the 'machines' slice is empty), it is not used for filtering.
func (c *Config) filterAndExpandConfigEntrys() error {

	sshConfig, err := LoadSshConfig()
	if err != nil {
		return err
	}

	flakesFilter := c.Global.Filters.Flakes
	configurationsFilter := c.Global.Filters.Configurations
	machinesFilter := c.Global.Filters.Machines

	for flakeName, flake := range c.Flakes.Range(false) {
		if (len(flakesFilter) > 0 && !slices.Contains(flakesFilter, flakeName)) || flake.Disabled {
			delete(c.Flakes, flakeName)
			continue
		}

		flake.Init(flakeName)

		for configurationName, configuration := range flake.Configurations.Range(false) {
			if (len(configurationsFilter) > 0 && !slices.Contains(configurationsFilter, configurationName)) || configuration.Disabled {
				delete(flake.Configurations, configurationName)
				continue
			}

			configuration.Phases = &ConfigurationPhases{
				Build: &PhaseBuild{},
			}

			configuration.Init(configurationName)

			for machineName, machine := range configuration.Machines.Range(false) {
				if machine == nil {
					machine = &Machine{}
					configuration.Machines[machineName] = machine
				}

				if machine.Ssh == nil {
					machine.Ssh = &SshClient{}
				}

				machine.Phases = &MachinePhases{
					Status: &PhaseStatus{},
				}

				machine.Init(machineName)

				if (len(machinesFilter) > 0 && !slices.Contains(machinesFilter, machineName)) || machine.Disabled {
					delete(configuration.Machines, machineName)
					continue
				}

				// A machine must match the tag filters if they are provided.
				if len(c.Global.Filters.Tags) > 0 {
					allMachineTags := flake.Tags
					allMachineTags = append(allMachineTags, configuration.Tags...)
					allMachineTags = append(allMachineTags, machine.Tags...)

					if !matchesTags(allMachineTags, c.Global.Filters.Tags) {
						delete(configuration.Machines, machineName)
						continue
					}
				}

				err := machine.Ssh.Validate(sshConfig, machineName)
				if err != nil {
					return err
				}
			}

			if len(configuration.Machines) == 0 {
				delete(flake.Configurations, configurationName)
			}
		}

		if len(flake.Configurations) == 0 {
			delete(c.Flakes, flakeName)
		}
	}

	if len(c.Flakes) == 0 {
		return fmt.Errorf("flakes configuration empty after filtering")
	}

	return nil
}

// Helpers

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
