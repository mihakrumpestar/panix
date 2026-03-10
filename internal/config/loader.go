package config

import (
	"os"
	"slices"
	"strconv"

	"github.com/goccy/go-yaml"
	"github.com/gookit/goutil/dump"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var ErrNoFlakesAfterFilter = errors.New("no flakes left after filtering")

func LoadConfig(parsedFlags flags.Flags, commandPhases []phases.Phase) (*Config, error) {
	dump.Config(func(d *dump.Options) {
		d.BytesAsString = true
		d.SkipNilField = true
		d.MaxDepth = 99
	})

	// Load YAML config file using streaming
	conf, err := decodeConfigFile(parsedFlags.Config)
	if err != nil {
		return nil, err
	}

	// Apply defaults and merge with CLI flags
	err = applyConfigDefaults(conf, parsedFlags)
	if err != nil {
		return nil, err
	}

	// Validate and initialize configuration
	err = validateAndInitConfig(conf)
	if err != nil {
		return nil, err
	}

	// Filter based on tags and disabled flags
	err = conf.filterRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to filter config")
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

// decodeConfigFile opens and decodes the YAML configuration file.
func decodeConfigFile(configPath string) (*Config, error) {
	//nolint:gosec // Config path is user-provided configuration file path by design
	file, err := os.Open(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed opening config %s", strconv.Quote(configPath))
	}

	defer func() {
		err := file.Close()
		if err != nil {
			log.Error().Err(err).Msg("failed to close config file")
		}
	}()

	conf := &Config{}
	decoder := yaml.NewDecoder(file)

	err = decoder.Decode(conf)
	if err != nil {
		return nil, errors.New(yaml.FormatError(err, true, false))
	}

	return conf, nil
}

// applyConfigDefaults merges configuration with CLI flags and applies defaults.
func applyConfigDefaults(conf *Config, parsedFlags flags.Flags) error {
	// Apply defaults
	if conf.Flags == nil {
		conf.Flags = &flags.Flags{}
	}

	err := conf.Flags.MergeConfWithCliFlags(parsedFlags)
	if err != nil {
		return errors.Wrap(err, "failed merging config with cli flags")
	}

	if conf.ColorScheme == nil {
		conf.ColorScheme = defaultColorScheme()
	}

	err = flags.InitLogging(conf.Flags.Logging)
	if err != nil {
		return errors.Wrap(err, "failed to initialize logging")
	}

	return nil
}

// validateAndInitConfig validates the configuration and initializes all entities.
func validateAndInitConfig(conf *Config) error {
	err := conf.ValidateStructTags()
	if err != nil {
		return errors.Wrap(err, "invalid configuration")
	}

	err = conf.initRoot()
	if err != nil {
		return errors.Wrap(err, "invalid configuration")
	}

	return nil
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

// filterRoot filters the configuration based on command-line or global selections.
func (c *Config) filterRoot() error {
	for _, flakePair := range c.Root.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		if flake == nil || flake.Disabled || flake.Configurations == nil {
			_, _ = c.Root.Flakes.Omap.Del(flakePair.Key)

			continue
		}

		c.filterFlakeConfigurations(flake)

		// Delete flake if no configs left
		if flake.Configurations.Omap.Len() == 0 {
			_, _ = c.Root.Flakes.Omap.Del(flakePair.Key)
		}
	}

	if c.Root.Flakes.Omap.Len() == 0 {
		return ErrNoFlakesAfterFilter
	}

	return nil
}

// filterFlakeConfigurations removes disabled or empty configurations from a flake.
func (c *Config) filterFlakeConfigurations(flake *Flake) {
	for _, configPair := range flake.Configurations.Omap.Pairs() {
		config := configPair.Value
		if config == nil || config.Disabled || config.Machines == nil {
			_, _ = flake.Configurations.Omap.Del(configPair.Key)

			continue
		}

		c.filterConfigurationMachines(config)

		// Delete config if no machines left
		if config.Machines.Omap.Len() == 0 {
			_, _ = flake.Configurations.Omap.Del(configPair.Key)
		}
	}
}

// filterConfigurationMachines removes disabled or untagged machines from a configuration.
func (c *Config) filterConfigurationMachines(config *Configuration) {
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
