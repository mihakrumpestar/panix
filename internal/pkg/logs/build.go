package logs

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
)

func InitBuildLogs(root *config.Fleet, logging flags.Logging) (*TargetsLogs, error) {
	targetsLogs, err := NewTargetsLogs(logging)
	if err != nil {
		return nil, err
	}

	for _, pair := range root.Flakes.Omap.Pairs() {
		flake := pair.Value

		var flakeLogs *TargetLogs

		flakeLogs, err = targetsLogs.Add(flake.Xpath)
		if err != nil {
			return nil, err
		}

		for _, configPair := range flake.Configurations.Omap.Pairs() {
			configuration := configPair.Value

			var configurationLogs *TargetLogs

			configurationLogs, err = targetsLogs.AddWithParent(configuration.Xpath, flakeLogs)
			if err != nil {
				return nil, err
			}

			for _, machinePair := range configuration.Machines.Omap.Pairs() {
				machine := machinePair.Value

				_, err = targetsLogs.AddWithParent(machine.Xpath, configurationLogs)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return targetsLogs, nil
}
