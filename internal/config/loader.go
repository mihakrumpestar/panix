package config

import (
	"os"
	"strconv"
	"time"

	"github.com/gookit/goutil/dump"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/template"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/pkg/yamlx"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

func LoadConfig(parsedFlags flags.Flags, commandPhases []phase.Phase) (*Config, error) {
	dump.Config(func(d *dump.Options) {
		d.BytesAsString = true
		d.SkipNilField = true
		d.MaxDepth = 99
	})

	// Load YAML config file using streaming
	conf, err := decodeConfigFile(parsedFlags.Config)
	if err != nil {
		return nil, err
	}

	// Apply defaults and merge with CLI flags
	err = applyConfigDefaults(conf, parsedFlags)
	if err != nil {
		return nil, err
	}

	// Validate and initialize configuration
	err = conf.ValidateStructTags()
	if err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	err = conf.initFleet()
	if err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	// Filter based on tags and disabled flags
	err = conf.Fleet.Filter(conf.Flags)
	if err != nil {
		return nil, errors.Wrap(err, "failed to filter config")
	}

	conf.Phases, err = phase.ValidatePhases(commandPhases, conf.Flags.SkipPhases)
	if err != nil {
		return nil, errors.Wrap(err, "invalid phases")
	}

	conf.filterOutUnusedPhases()

	if conf.Flags.Logging.Debug {
		dump.P(conf.Flags)
		dump.P(conf.Fleet)
	}

	conf.Snapshot.StartTime = time.Now()
	conf.Snapshot.PanixVersion = gen.Version()

	return conf, nil
}

// decodeConfigFile opens and decodes the YAML configuration file.
func decodeConfigFile(configPath string) (*Config, error) {
	//nolint:gosec // Config path is user-provided configuration file path by design
	rawYAML, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading config %s", strconv.Quote(configPath))
	}

	// Process templates before decoding
	processedYAML, err := template.ProcessTemplate(rawYAML)
	if err != nil {
		return nil, errors.Wrap(err, "failed to process templates in config")
	}

	conf := &Config{}

	err = yamlx.Decode(processedYAML, conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode config")
	}

	return conf, nil
}

// applyConfigDefaults merges configuration with CLI flags and applies defaults.
func applyConfigDefaults(conf *Config, parsedFlags flags.Flags) error {
	err := conf.Flags.MergeConfWithCliFlags(parsedFlags)
	if err != nil {
		return errors.Wrap(err, "failed merging config with cli flags")
	}

	if conf.ColorScheme == nil {
		conf.ColorScheme = colorscheme.DefaultColorScheme()
	}

	if conf.Flags.LocalMachineHostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return errors.Wrap(err, "failed to get hostname")
		}

		conf.Flags.LocalMachineHostname = hostname
	}

	conf.Flags.DefautlIfNoTTY()

	err = logger.InitLogging(conf.Flags.Logging, conf.Flags.Output)
	if err != nil {
		return errors.Wrap(err, "failed to initialize logging")
	}

	return nil
}

func (c *Config) initFleet() error {
	localMachineHostname := c.Flags.LocalMachineHostname

	err := c.Fleet.Init(localMachineHostname)
	if err != nil {
		return errors.Wrap(err, "failed to init fleet")
	}

	for _, flakePair := range c.Fleet.Flakes.Pairs() {
		flakeV := flakePair.Value

		err = flakeV.Init(flakePair.Key, &c.Fleet.Attributes, localMachineHostname)
		if err != nil {
			return errors.Wrap(err, "failed to init flake")
		}

		for _, configurationPair := range flakeV.Configurations.Pairs() {
			configurationV := configurationPair.Value

			err = configurationV.Init(configurationPair.Key, &flakeV.Attributes, localMachineHostname)
			if err != nil {
				return errors.Wrap(err, "failed to init configuration")
			}

			for _, machinePair := range configurationV.Machines.Pairs() {
				machineV := machinePair.Value

				// Machine may be nil
				if machineV == nil {
					machineV = &machine.Machine{}
					configurationV.Machines.Set(machinePair.Key, machineV)
				}

				err = machineV.Init(machinePair.Key, &configurationV.Attributes, localMachineHostname)
				if err != nil {
					return errors.Wrap(err, "failed to init machine")
				}
			}
		}
	}

	return nil
}
