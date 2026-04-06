package logs

import (
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
)

func InitBuildLogs(flakes []*config.Flake, logging flags.Logging) (*TargetsLogs, error) {
	targetsLogs, err := NewTargetsLogs(logging)
	if err != nil {
		return nil, err
	}

	for _, flake := range flakes {
		flakeLogs, err := targetsLogs.Add(flake.Xpath)

		if err != nil {
			return nil, err
		}

		for _, configuration := range flake.Configurations {
			configurationLogs, err := targetsLogs.AddWithParent(configuration.Xpath, flakeLogs)

			if err != nil {
				return nil, err
			}

			for _, machine := range configuration.Machines {
				_, err := targetsLogs.AddWithParent(machine.Xpath, configurationLogs)

				if err != nil {
					return nil, err
				}
			}
		}
	}

	return targetsLogs, nil
}
