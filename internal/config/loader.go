package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"

	"github.com/goccy/go-yaml"
	"github.com/gookit/goutil/dump"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func LoadConfig(flags config_flags.Flags, commandPhases []phases.Phase) (*Config, error) {
	dump.Config(func(d *dump.Options) {
		d.BytesAsString = true
		d.SkipNilField = true
		d.MaxDepth = 99
	})

	// Load YAML config file using streaming
	file, err := os.Open(flags.Config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed opening config %s", strconv.Quote(flags.Config))
	}
	defer file.Close()

	conf := &Config{}
	decoder := yaml.NewDecoder(file)

	err = decoder.Decode(conf)
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

	err = config_flags.InitLogging(conf.Flags.Logging)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize logging")
	}

	err = conf.ValidateStructTags()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	err = conf.initRoot()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	err = conf.filterRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to filter config: %w", err)
	}

	conf.Phases, err = phases.ValidatePhases(commandPhases, conf.Flags.SkipPhases)
	if err != nil {
		return nil, errors.Wrap(err, "invalid phases")
	}

	if conf.Flags.Logging.Debug {
		dump.P(conf.Flags)
		dump.P(conf.Root)
	}

	return conf, nil
}

// Helper functions

func (c *Config) initRoot() error {
	err := c.Root.Init(c.Flags)
	if err != nil {
		return err
	}

	for _, flakePair := range c.Root.Flakes.Omap.Pairs() {
		flakeName, flake := flakePair.Key, flakePair.Value

		err := flake.Init(flakeName, &c.Root.Attributes)
		if err != nil {
			return err
		}

		for _, configPair := range flake.Configurations.Omap.Pairs() {
			configurationName, configuration := configPair.Key, configPair.Value

			err := configuration.Init(configurationName, flake)
			if err != nil {
				return err
			}

			for _, machinePair := range configuration.Machines.Omap.Pairs() {
				machineName, machine := machinePair.Key, machinePair.Value

				err = machine.Init(machineName, configuration)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// FilterConfigEntries filters the configuration based on command-line or global selections
func (c *Config) filterRoot() error {
	for _, flakePair := range c.Root.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		if flake == nil || flake.Disabled || flake.Configurations == nil {
			_, _ = c.Root.Flakes.Omap.Del(flakePair.Key)
			continue
		}

		for _, configPair := range flake.Configurations.Omap.Pairs() {
			config := configPair.Value
			if config == nil || config.Disabled || config.Machines == nil {
				_, _ = flake.Configurations.Omap.Del(configPair.Key)
				continue
			}

			// Filter machines
			for _, machinePair := range config.Machines.Omap.Pairs() {
				machine := machinePair.Value
				if machine == nil || machine.Disabled || !machineContainsTags(machine.Tags, c.Flags.Tags) {
					log.Debug().Bool("machine == nil", machine == nil).
						Bool("disabled", machine.Disabled).
						Strs("machine.Tags", machine.Tags).
						Msgf("deleting machine %s", strconv.Quote(machinePair.Key))

					_, _ = config.Machines.Omap.Del(machinePair.Key)
				}
			}

			// Delete config if no machines left
			if config.Machines.Omap.Len() == 0 {
				_, _ = flake.Configurations.Omap.Del(configPair.Key)
			}
		}

		// Delete flake if no configs left
		if flake.Configurations.Omap.Len() == 0 {
			_, _ = c.Root.Flakes.Omap.Del(flakePair.Key)
		}
	}

	if c.Root.Flakes.Omap.Len() == 0 {
		return fmt.Errorf("no flakes left after filtering")
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
