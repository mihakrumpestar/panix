package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/gookit/goutil/dump"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/cliflagv3"
	"github.com/knadh/koanf/providers/file"
	koanf "github.com/knadh/koanf/v2"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/urfave/cli/v3"
)

// LoadConfig reads configuration from multiple sources in priority order:
// 1. Config file (YAML) - lowest priority
// 2. Environment variables (PANIX_*) - via urfave/cli v3 flag sources
// 3. CLI flags - highest priority (passed via cmd parameter)
func LoadConfig(flags *config_flags.Flags, cmd *cli.Command) (*Config, error) {
	k := koanf.New(".")

	_, err := os.Stat(flags.Config)
	if err == nil {
		err = k.Load(file.Provider(flags.Config), yaml.Parser())
		if err != nil {
			return nil, fmt.Errorf("fatal error config file: %w", err)
		}
	}

	tmp := cliflagv3.Provider(cmd, "-")

	tmp2, err := tmp.Read()
	if err != nil {
		return nil, err
	}

	for name, flag := range tmp2 {
		fmt.Println(strconv.Quote(name), strconv.Quote(fmt.Sprintf("%v", flag)))
	}

	if cmd != nil {
		err = k.Load(tmp, nil)
		if err != nil {
			return nil, fmt.Errorf("error loading CLI flags: %w", err)
		}
	}

	// Unmarshal into a Config struct
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

	if conf.ColorScheme == nil {
		conf.ColorScheme = defaultColorScheme()
	}

	err = conf.initAndValidateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	err = conf.filterRootTree()
	if err != nil {
		return nil, fmt.Errorf("failed to filter config: %w", err)
	}

	err = conf.initLogs()
	if err != nil {
		return nil, fmt.Errorf("failed to init logs: %w", err)
	}

	if conf.Flags.Logging.Debug {
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

func (c *Config) initAndValidateConfig() error {
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

func (c *Config) initLogs() error {
	targetsLogs, err := logs.NewTargetsLogs(c.Flags.Logging)
	if err != nil {
		return err
	}

	c.TargetsLogs = targetsLogs

	for _, flake := range c.Root.Flakes.SortedMap() {
		flakeLogs, err := targetsLogs.Add(flake.Xpath)
		if err != nil {
			return err
		}

		for _, configuration := range flake.Configurations.SortedMap() {
			configurationLogs, err := targetsLogs.AddWithParent(configuration.Xpath, flakeLogs)
			if err != nil {
				return err
			}

			for _, machine := range configuration.Machines.SortedMap() {
				_, err := targetsLogs.AddWithParent(machine.Xpath, configurationLogs)
				if err != nil {
					return err
				}
			}
		}
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
