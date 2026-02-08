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

	if c.Root.Flakes.Len() == 0 {
		return fmt.Errorf("flakes is required")
	}

	for _, flakePair := range c.Root.Flakes.Omap.Pairs() {
		flakeName, flake := flakePair.Key, flakePair.Value
		if flake.URL == "" {
			return fmt.Errorf("flake %s has no URL configured", flakeName)
		}

		err := flake.Init(flakeName, &c.Root.Attributes, c.Flags)
		if err != nil {
			return err
		}

		if flake.Configurations.Len() == 0 {
			return fmt.Errorf("flakes[%s]configurations is empty", flakeName)
		}

		for _, configPair := range flake.Configurations.Omap.Pairs() {
			configurationName, configuration := configPair.Key, configPair.Value
			if configuration.Machines.Len() == 0 {
				return fmt.Errorf("flakes[%s]configurations[%s]machines is empty", flakeName, configurationName)
			}

			err := configuration.Init(configurationName, flake, c.Flags)
			if err != nil {
				return err
			}

			for _, machinePair := range configuration.Machines.Omap.Pairs() {
				machineName, machine := machinePair.Key, machinePair.Value

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
	// Collect keys to delete instead of modifying while iterating
	flakesToDelete := make([]string, 0)

	for _, flakePair := range c.Root.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		if flake == nil || flake.Configurations == nil {
			flakesToDelete = append(flakesToDelete, flakePair.Key)
			continue
		}

		configsToDelete := make([]string, 0)

		for _, configPair := range flake.Configurations.Omap.Pairs() {
			config := configPair.Value
			if config == nil || config.Disabled || config.Machines == nil {
				configsToDelete = append(configsToDelete, configPair.Key)
				continue
			}

			// Filter machines
			machinesToDelete := make([]string, 0)
			for _, machinePair := range config.Machines.Omap.Pairs() {
				machine := machinePair.Value
				if machine == nil || machine.Disabled || !machineContainsTags(machine.Tags, c.Flags.Tags) {
					machinesToDelete = append(machinesToDelete, machinePair.Key)
				}
			}

			for _, key := range machinesToDelete {
				config.Machines.Omap.Del(key)
			}

			// Delete config if no machines left
			if config.Machines.Omap.Len() == 0 {
				configsToDelete = append(configsToDelete, configPair.Key)
			}
		}

		for _, key := range configsToDelete {
			flake.Configurations.Omap.Del(key)
		}

		// Delete flake if no configs left
		if flake.Configurations.Omap.Len() == 0 {
			flakesToDelete = append(flakesToDelete, flakePair.Key)
		}
	}

	for _, key := range flakesToDelete {
		c.Root.Flakes.Omap.Del(key)
	}

	if c.Root.Flakes.Omap.Len() == 0 {
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

	for _, pair := range c.Root.Flakes.Omap.Pairs() {
		flake := pair.Value
		flakeLogs, err := targetsLogs.Add(flake.Xpath)
		if err != nil {
			return err
		}

		for _, configPair := range flake.Configurations.Omap.Pairs() {
			configuration := configPair.Value
			configurationLogs, err := targetsLogs.AddWithParent(configuration.Xpath, flakeLogs)
			if err != nil {
				return err
			}

			for _, machinePair := range configuration.Machines.Omap.Pairs() {
				machine := machinePair.Value
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
