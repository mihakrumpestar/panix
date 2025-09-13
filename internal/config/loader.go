package config

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/elliotchance/orderedmap/v3"
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

	// First, unmarshal into simplifiedConfig structure with regular maps
	simplifiedConfig := &decodingConfig{}
	err = k.UnmarshalWithConf("", &simplifiedConfig, koanf.UnmarshalConf{
		Tag: "yaml",
		DecoderConfig: &mapstructure.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           &simplifiedConfig,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	// Convert timeout from miliseconds to seconds for duration
	simplifiedConfig.Global.Timeout *= time.Second

	// Convert simplified structure to final structure with ordered maps
	conf, err := simplifiedConfig.convertToFinalConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to convert config: %w", err)
	}

	err = conf.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	conf.Flakes, err = conf.filterAndExpandConfigEntrys()
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
	if c.Flakes.Len() == 0 {
		return fmt.Errorf("flakes is required")
	}

	for flakeName, flake := range c.Flakes.AllFromFront() {
		if flake.Url == "" {
			return fmt.Errorf("flake %s has no URL configured", flakeName)
		}

		if flake.Configurations == nil || flake.Configurations.Len() == 0 {
			return fmt.Errorf("flakes[%s]configurations is empty", flakeName)
		}

		for configurationName, configuration := range flake.Configurations.AllFromFront() {
			if configuration.Machines == nil || configuration.Machines.Len() == 0 {
				return fmt.Errorf("flakes[%s]configurations[%s]machines is empty", flakeName, configurationName)
			}

			for machineName := range configuration.Machines.AllFromFront() {
				if machineName.Host == "" {
					return fmt.Errorf("flakes[%s]configurations[%s]machines[%s].ssh host is empty", flakeName, configurationName, machineName.String())
				}
			}
		}
	}

	return nil
}

// FilterConfigEntrys filters the configuration based on command-line or global selections.
// An entry is kept if it matches all provided filters (flakes, configurations, machines, tags) and is not disabled.
// If a filter type is not provided (e.g., the 'machines' slice is empty), it is not used for filtering.
func (c *Config) filterAndExpandConfigEntrys() (*orderedmap.OrderedMap[string, *Flake], error) {
	cC := *c // Copy

	sc, err := LoadSshConfig()
	if err != nil {
		return nil, err
	}

	flakesFilter := cC.Global.Filters.Flakes
	configurationsFilter := cC.Global.Filters.Configurations
	machinesFilter := cC.Global.Filters.Machines

	for flakeName, flake := range cC.Flakes.AllFromFront() {
		if (len(flakesFilter) > 0 && !slices.Contains(flakesFilter, flakeName)) || flake.Disabled {
			cC.Flakes.Delete(flakeName)
			continue
		}

		flake.Logs = NewLogs()

		for configurationName, configuration := range flake.Configurations.AllFromFront() {
			if (len(configurationsFilter) > 0 && !slices.Contains(configurationsFilter, configurationName)) || configuration.Disabled {
				flake.Configurations.Delete(configurationName)
				continue
			}

			configuration.Phases = &ConfigurationPhases{
				Build: &PhaseBuild{},
			}

			configuration.Logs = NewLogs()

			for machineName, machine := range configuration.Machines.AllFromFront() {
				if machine == nil {
					machine = &Machine{}
					configuration.Machines.Set(machineName, machine)
				}

				machine.Phases = &MachinePhases{
					Status: &PhaseStatus{},
				}

				machine.Logs = NewLogs()

				if (len(machinesFilter) > 0 && !slices.Contains(machinesFilter, machineName.String())) || machine.Disabled {
					configuration.Machines.Delete(machineName)
					continue
				}

				// A machine must match the tag filters if they are provided.
				if len(cC.Global.Filters.Tags) > 0 {
					allMachineTags := flake.Tags
					allMachineTags = append(allMachineTags, configuration.Tags...)
					allMachineTags = append(allMachineTags, machine.Tags...)

					if !matchesTags(allMachineTags, cC.Global.Filters.Tags) {
						configuration.Machines.Delete(machineName)
						continue
					}
				}

				if machineName.User.Username() == "" { // If using alias, we need to retrive ssh config values
					if machine.Ssh == nil {
						machine.Ssh = &SshClient{}
					}
					machine.Ssh.Url, err = sc.RetriveFullParamsFromSshConfig(machineName)
					if err != nil {
						return nil, err
					}
				}
			}

			if configuration.Machines.Len() == 0 {
				flake.Configurations.Delete(configurationName)
			}
		}

		if flake.Configurations.Len() == 0 {
			cC.Flakes.Delete(flakeName)
		}
	}

	if cC.Flakes.Len() == 0 {
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
