package config

import (
	"fmt"
	"slices"
	"strings"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/gobeam/stringy"
	"github.com/gookit/goutil/dump"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	koanf "github.com/knadh/koanf/v2"
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
		// Debug: Print all flags before processing
		//fmt.Printf("DEBUG: Processing flags:\n")
		//flags.VisitAll(func(f *pflag.Flag) {
		//	fmt.Printf("DEBUG: Flag name: %s, value: %s\n", f.Name, f.Value.String())
		//})

		err := k.Load(
			posflag.ProviderWithFlag(
				flags,
				".",
				k,
				func(f *pflag.Flag) (string, any) {
					// Skip non-configuration flags
					if slices.Contains([]string{"config", "help"}, f.Name) {
						return "", nil // Skip this flag
					}

					// Transform the key in whatever manner.
					keyRaw := stringy.New(f.Name) //.CamelCase()

					key := keyRaw.Prefix("flags.")

					val := posflag.FlagVal(flags, f)

					// Debug: Print the transformation
					//fmt.Printf("DEBUG: Transforming flag '%s' to key '%s' with value '%v'\n", f.Name, key, val)

					return key, val
				},
			),
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("error loading command line flags: %w", err)
		}
	}

	// Unmarshal conf
	conf := &Config{}
	err = k.UnmarshalWithConf("", &conf, koanf.UnmarshalConf{
		Tag:       "yaml",
		FlatPaths: false,
		DecoderConfig: &mapstructure.DecoderConfig{
			ErrorUnused:          true,
			WeaklyTypedInput:     true,
			Squash:               true,
			IgnoreUntaggedFields: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	conf.Flags.Setup()

	// TODO: implement file loader than can parse custom configs
	if conf.Tui == nil {
		conf.Tui = &Tui{}
	}

	conf.Tui.ColorScheme = defaultColorScheme()

	err = conf.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	err = conf.filterRootTree()
	if err != nil {
		return nil, fmt.Errorf("failed to filter config: %w", err)
	}

	if conf.Flags.Debug {
		dump.Config(func(d *dump.Options) {
			d.BytesAsString = true
			d.SkipNilField = true
			d.MaxDepth = 99
		})

		dump.P(conf)
	}

	return conf, nil
}

// Helper functions

func (c *Config) validateConfig() error {
	if c.Root == nil {
		return fmt.Errorf("root is nil")
	}

	err := c.Root.Init()
	if err != nil {
		return err
	}

	if len(c.Root.Flakes) == 0 {
		return fmt.Errorf("flakes is required")
	}

	for flakeName, flake := range c.Root.Flakes.SortedMap() {
		if flake.Url == "" {
			return fmt.Errorf("flake %s has no URL configured", flakeName)
		}

		err := flake.Init(flakeName, &c.Root.Attributes, c.Flags)
		if err != nil {
			return err
		}

		if len(flake.Configurations) == 0 {
			return fmt.Errorf("flakes[%s]configurations is empty", flakeName)
		}

		for configurationName, configuration := range flake.Configurations.SortedMap() {
			if len(configuration.Machines) == 0 {
				return fmt.Errorf("flakes[%s]configurations[%s]machines is empty", flakeName, configurationName)
			}

			err := configuration.Init(configurationName, flake, c.Flags)
			if err != nil {
				return err
			}

			for machineName, machine := range configuration.Machines.SortedMap() {
				if machine == nil {
					machine = &Machine{}
					configuration.Machines[machineName] = machine
				}

				err = machine.Init(machineName, configuration, c.Flags)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// FilterConfigEntrys filters the configuration based on command-line or global selections
func (c *Config) filterRootTree() error {
	for flakeName, flake := range c.Root.Flakes.SortedMap() {
		if flake.Disabled {
			delete(c.Root.Flakes, flakeName)
			continue
		}

		for configurationName, configuration := range flake.Configurations.SortedMap() {
			if configuration.Disabled {
				delete(flake.Configurations, configurationName)
				continue
			}

			for machineName, machine := range configuration.Machines.SortedMap() {
				if machine.Disabled {
					delete(configuration.Machines, machineName)
					continue
				}

				if !machineContainesTags(machine.Tags, c.Flags.Tags) {
					delete(configuration.Machines, machineName)
					continue
				}
			}

			if len(configuration.Machines) == 0 {
				delete(flake.Configurations, configurationName)
			}
		}

		if len(flake.Configurations) == 0 {
			delete(c.Root.Flakes, flakeName)
		}
	}

	if len(c.Root.Flakes) == 0 {
		return fmt.Errorf("no flakes left after filtering")
	}

	return nil
}

// Helpers

func machineContainesTags(tags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	for _, filterTag := range filterTags {
		if slices.Contains(tags, filterTag) {
			return true
		}
	}

	return false
}
