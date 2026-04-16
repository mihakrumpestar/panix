package configuration

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/rs/zerolog/log"
)

func (c *Configuration) Filter(flags flags.Flags) {
	c.Machines.DeleteFunc(func(name string, machineI *machine.Machine) bool {
		if machineI == nil || machineI.Disabled || !machineContainsTags(machineI.Tags, flags.Tags) {
			log.Debug().Bool("machine == nil", machineI == nil).
				Bool("disabled", machineI != nil && machineI.Disabled).
				Strs("machine.Tags", machineI.Tags).
				Msgf("deleting machine %s", machineI.Name)

			return true
		}

		return false
	})
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
