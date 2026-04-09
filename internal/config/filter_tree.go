package config

import (
	"strconv"

	"github.com/rs/zerolog/log"
)

// filterFleet filters the configuration based on command-line or global selections.
func (c *Config) filterFleet() error {
	for _, flakePair := range c.Fleet.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		if flake == nil || flake.Disabled || flake.Configurations == nil {
			_, _ = c.Fleet.Flakes.Omap.Del(flakePair.Key)

			continue
		}

		c.filterFlakeConfigurations(flake)

		// Delete flake if no configs left
		if flake.Configurations.Omap.Len() == 0 {
			_, _ = c.Fleet.Flakes.Omap.Del(flakePair.Key)
		}
	}

	if c.Fleet.Flakes.Omap.Len() == 0 {
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
