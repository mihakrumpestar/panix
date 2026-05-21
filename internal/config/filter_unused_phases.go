package config

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/phase"
)

type optionalPhases struct {
	Secrets   bool
	Bootstrap bool
}

func (c *Config) FilterOutUnusedPhases() {
	hasRequiredPhases := hasRequiredPhases(c.Fleet)

	c.Phases = slices.DeleteFunc(slices.Clone(c.Phases), func(p phase.Phase) bool {
		return (p == phase.Secrets && !hasRequiredPhases.Secrets) ||
			(p == phase.Bootstrap && !hasRequiredPhases.Bootstrap)
	})
}

func hasRequiredPhases(f *fleet.Fleet) optionalPhases {
	var has optionalPhases

	for _, treeLeaf := range f.AllMachines() {
		m := treeLeaf.Machine
		if len(m.Secrets) > 0 {
			has.Secrets = true
		}

		if m.Bootstrap.SSH.IsInitialized() || m.Bootstrap.ForceBootstrap {
			has.Bootstrap = true
		}

		// Early return since we already have all possible optional phases
		if has.Secrets && has.Bootstrap {
			break
		}
	}

	return has
}
