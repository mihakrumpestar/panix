package config

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type hasPhases struct {
	Secrets   bool
	Bootstrap bool
}

func (c *Config) filterUnusedPhases() {
	hasRequiredPhases := c.hasRequiredPhases()

	c.Phases = slices.DeleteFunc(slices.Clone(c.Phases), func(phase phases.Phase) bool {
		return (phase == phases.Secrets && !hasRequiredPhases.Secrets) ||
			(phase == phases.Bootstrap && !hasRequiredPhases.Bootstrap)
	})
}

func (c *Config) hasRequiredPhases() hasPhases {
	var has hasPhases

	for _, flakePair := range c.Fleet.Flakes.Pairs() {
		for _, cfgPair := range flakePair.Value.Configurations.Pairs() {
			for _, machinePair := range cfgPair.Value.Machines.Pairs() {
				machine := machinePair.Value

				if len(machine.Secrets) > 0 {
					has.Secrets = true
				}

				if machine.Bootstrap.SSH != nil || machine.Bootstrap.ForceBootstrap {
					has.Bootstrap = true
				}

				if has.Secrets && has.Bootstrap {
					return has
				}
			}
		}
	}

	return has
}
