package config

import (
	"fmt"
	"net/url"

	"github.com/elliotchance/orderedmap/v3"
)

// Simplified structures for unmarshalling with regular maps
type decodingConfig struct {
	Global Global                    `koanf:"global"`
	Flakes map[string]*decodingFlake `koanf:"flakes"`
}

type decodingFlake struct {
	Url               string `koanf:"url"`
	DefaultAttributes `koanf:",squash"`
	Configurations    map[string]*decodingConfiguration `koanf:"configurations"`
}

type decodingConfiguration struct {
	FlakeOutput       string `koanf:"flakeOutput"`
	DefaultAttributes `koanf:",squash"`
	Machines          map[string]*decodingMachine `koanf:"machines"`
}

type decodingMachine struct {
	DefaultAttributes `koanf:",squash"`
}

// convertToFinalConfig converts the simplified config structure to the final structure with ordered maps
func (sc *decodingConfig) convertToFinalConfig() (*Config, error) {
	final := &Config{
		Global: sc.Global,
		Flakes: orderedmap.NewOrderedMap[string, *Flake](),
	}

	// Convert flakes
	for flakeName, simplifiedFlake := range sc.Flakes {
		flake := &Flake{
			Url:               simplifiedFlake.Url,
			DefaultAttributes: simplifiedFlake.DefaultAttributes,
			Configurations:    orderedmap.NewOrderedMap[string, *Configuration](),
		}

		final.Flakes.Set(flakeName, flake)

		// Convert configurations
		for configName, simplifiedConfig := range simplifiedFlake.Configurations {
			configuration := &Configuration{
				FlakeOutput:       simplifiedConfig.FlakeOutput,
				DefaultAttributes: simplifiedConfig.DefaultAttributes,
				Machines:          orderedmap.NewOrderedMap[url.URL, *Machine](),
			}

			flake.Configurations.Set(configName, configuration)

			// Convert machines
			for machineName, simplifiedMachine := range simplifiedConfig.Machines {
				parsedMachineName, err := url.ParseRequestURI("ssh://" + machineName)
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
