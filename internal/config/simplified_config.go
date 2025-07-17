package config

import (
	"fmt"
	"net/url"

	"github.com/elliotchance/orderedmap/v3"
)

// Simplified structures for unmarshalling with regular maps
type simplifiedConfig struct {
	Global Global                      `koanf:"global"`
	Flakes map[string]*simplifiedFlake `koanf:"flakes"`
}

type simplifiedFlake struct {
	Url             string `koanf:"url"`
	treeStyleParams `koanf:",squash"`
	Configurations  map[string]*simplifiedConfiguration `koanf:"configurations"`
}

type simplifiedConfiguration struct {
	FlakeOutput     string `koanf:"flakeOutput"`
	treeStyleParams `koanf:",squash"`
	Machines        map[string]*simplifiedMachine `koanf:"machines"`
}

type simplifiedMachine struct {
	treeStyleParams `koanf:",squash"`
}

// convertToFinalConfig converts the simplified config structure to the final structure with ordered maps
func (sc *simplifiedConfig) convertToFinalConfig() (*Config, error) {
	final := &Config{
		Global: sc.Global,
		Flakes: orderedmap.NewOrderedMap[string, *Flake](),
	}

	// Convert flakes
	for flakeName, simplifiedFlake := range sc.Flakes {
		flake := &Flake{
			Url:             simplifiedFlake.Url,
			treeStyleParams: simplifiedFlake.treeStyleParams,
			Configurations:  orderedmap.NewOrderedMap[string, *Configuration](),
		}

		final.Flakes.Set(flakeName, flake)

		// Convert configurations
		for configName, simplifiedConfig := range simplifiedFlake.Configurations {
			configuration := &Configuration{
				FlakeOutput:     simplifiedConfig.FlakeOutput,
				treeStyleParams: simplifiedConfig.treeStyleParams,
				Machines:        orderedmap.NewOrderedMap[url.URL, *Machine](),
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
					machine.treeStyleParams = simplifiedMachine.treeStyleParams
				}

				configuration.Machines.Set(*parsedMachineName, machine)
			}

		}
	}

	return final, nil
}
