package config

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

type hasPhases struct {
	Secrets   bool
	Bootstrap bool
}

func (c *Config) filterUnusedPhases() {
	hasRequiredPhases := c.hasRequiredPhases()

	c.Phases = slices.DeleteFunc(slices.Clone(c.Phases), func(p phase.Phase) bool {
		return (p == phase.Secrets && !hasRequiredPhases.Secrets) ||
			(p == phase.Bootstrap && !hasRequiredPhases.Bootstrap)
	})
}

func (c *Config) hasRequiredPhases() hasPhases {
	var has hasPhases

	for _, treeLeaf := range c.Fleet.AllMachines() {
		m := treeLeaf.Machine
		if len(m.Secrets) > 0 {
			has.Secrets = true
		}

		if m.Bootstrap.SSH != nil || m.Bootstrap.ForceBootstrap {
			has.Bootstrap = true
		}

		// Early return since we already have all possible optional phases
		if has.Secrets && has.Bootstrap {
			break
		}
	}

	return has
}
