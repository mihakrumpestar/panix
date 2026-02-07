package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"

	"github.com/goccy/go-yaml"
	"github.com/gookit/goutil/dump"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/pkg/errors"
)

func LoadConfig(flags config_flags.Flags) (*Config, error) {
	dump.Config(func(d *dump.Options) {
		d.BytesAsString = true
		d.SkipNilField = true
		d.MaxDepth = 99
	})

	// Load YAML config file
	confRaw, err := os.ReadFile(flags.Config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading config %s", strconv.Quote(flags.Config))
	}

	conf := &Config{}
	err = yaml.Unmarshal(confRaw, conf)
	if err != nil {
		return nil, errors.New(yaml.FormatError(err, true, false))
	}

	err = conf.Flags.MergeConfWithCliFlags(flags)
	if err != nil {
		return nil, errors.Wrap(err, "failed merging config with cli flags")
	}

	// Apply defaults
	if conf.Flags == nil {
		conf.Flags = &config_flags.Flags{}
	}

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
		dump.P(conf.Flags)
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
		if flake.URL == "" {
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

// FilterConfigEntries filters the configuration based on command-line or global selections
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

				if !machineContainsTags(machine.Tags, c.Flags.Tags) {
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

func machineContainsTags(tags, filterTags []string) bool {
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
