package config

import (
	"fmt"
	"maps"
	"net/url"
	"slices"

	"github.com/elliotchance/orderedmap/v3"
)

// Simplified structures for unmarshalling with regular maps
type decodingConfig struct {
	Global Global                    `yaml:"global"`
	Flakes map[string]*decodingFlake `yaml:"flakes"`
}

type decodingFlake struct {
	Url               string `yaml:"url"`
	DefaultAttributes `yaml:",inline"`
	Configurations    map[string]*decodingConfiguration `yaml:"configurations"`
	BuildHooks        BuildHooks                        `yaml:",buildHooks"` // They only run for builds
}

type decodingConfiguration struct {
	FlakeOutput       string `yaml:"flakeOutput"`
	DefaultAttributes `yaml:",inline"`
	Machines          map[string]*decodingMachine `yaml:"machines"`
}

type decodingMachine struct {
	DefaultAttributes `yaml:",inline"`
}

// convertToFinalConfig converts the simplified config structure to the final structure with ordered maps
func (sc *decodingConfig) convertToFinalConfig() (*Config, error) {
	final := &Config{
		Global: sc.Global,
		Flakes: orderedmap.NewOrderedMap[string, *Flake](),
	}

	// Convert flakes
	for _, flakeName := range slices.Sorted(maps.Keys(sc.Flakes)) {
		simplifiedFlake := sc.Flakes[flakeName]

		flake := &Flake{
			Url:               simplifiedFlake.Url,
			DefaultAttributes: simplifiedFlake.DefaultAttributes,
			Configurations:    orderedmap.NewOrderedMap[string, *Configuration](),
			BuildHooks:        simplifiedFlake.BuildHooks,
		}

		final.Flakes.Set(flakeName, flake)

		// Convert configurations
		for _, configName := range slices.Sorted(maps.Keys(simplifiedFlake.Configurations)) {
			simplifiedConfiguration := simplifiedFlake.Configurations[configName]

			configuration := &Configuration{
				FlakeOutput:       simplifiedConfiguration.FlakeOutput,
				DefaultAttributes: simplifiedConfiguration.DefaultAttributes,
				Machines:          orderedmap.NewOrderedMap[url.URL, *Machine](),
			}

			flake.Configurations.Set(configName, configuration)

			// Convert machines
			for _, machineName := range slices.Sorted(maps.Keys(simplifiedConfiguration.Machines)) {
				simplifiedMachine := simplifiedConfiguration.Machines[machineName]

				parsedMachineName, err := url.Parse("ssh://" + machineName)
				if err != nil {
					return nil, fmt.Errorf("invalid machine name %s, has to be formatted as URL: %w", machineName, err)
				}

				machine := &Machine{}
				if simplifiedMachine != nil {
					machine.DefaultAttributes = simplifiedMachine.DefaultAttributes
				}

				configuration.Machines.Set(*parsedMachineName, machine)
			}

		}
	}

	return final, nil
}
