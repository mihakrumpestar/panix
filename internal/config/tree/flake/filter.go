package flake

import (
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
)

func (f *Flake) Filter() {
	f.Configurations.DeleteFunc(func(name string, configurationI *configuration.Configuration) bool {
		if configurationI == nil || configurationI.Disabled {
			return true
		}

		configurationI.Filter()

		return configurationI.Machines.Len() == 0
	})
}
