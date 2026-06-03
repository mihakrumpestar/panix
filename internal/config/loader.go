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
	"github.com/mihakrumpestar/panix/internal/config/validate"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func LoadConfig(parsedFlags flags.Flags) (*Config, error) {
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

	err = conf.initFleet()
	if err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	// Filter based on tags and disabled flags
	err = conf.Fleet.Filter(conf.Flags)
	if err != nil {
		return nil, errors.Wrap(err, "failed to filter config")
	}

	// Detect nix implementation before SSH init so it can be injected into SSH clients.
	conf.Nix, err = nixver.Detect()
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect nix implementation")
	}

	log.Info().Str("nix", conf.Nix.Raw).Msg("detected nix implementation")

	// Initialize SSH for remaining machines after filtering.
	err = conf.initFleetSSH()
	if err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	// Validate configuration
	err = validate.ValidateStructTags(conf, conf.Fleet, conf.Flags.ValidateFlags)
	if err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

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
		var hostname string

		hostname, err = os.Hostname()
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
	err := c.Fleet.Init()
	if err != nil {
		return errors.Wrap(err, "failed to init fleet")
	}

	for _, flakePair := range c.Fleet.Flakes.Pairs() {
		flakeV := flakePair.Value

		err = flakeV.Init(flakePair.Key, &c.Fleet.Attributes, &c.Fleet.Nix)
		if err != nil {
			return errors.Wrap(err, "failed to init flake")
		}

		for _, configurationPair := range flakeV.Configurations.Pairs() {
			configurationV := configurationPair.Value

			err = configurationV.Init(configurationPair.Key, &flakeV.Attributes, &flakeV.Nix)
			if err != nil {
				return errors.Wrap(err, "failed to init configuration")
			}

			for _, machinePair := range configurationV.Machines.Pairs() {
				machineV := machinePair.Value

				// Machine may be nil due to existing only as key (this is intended behaviour), so we set it here in that case
				if machineV == nil {
					machineV = &machine.Machine{}

					configurationV.Machines.Set(machinePair.Key, machineV)
				}

				err = machineV.Init(machinePair.Key, &configurationV.Attributes)
				if err != nil {
					return errors.Wrap(err, "failed to init machine")
				}
			}
		}
	}

	return nil
}

// initFleetSSH initializes SSH configuration for all remaining machines after filtering.
// This is separated from initFleet so that filtered-out machines never trigger SSH config loading.
func (c *Config) initFleetSSH() error {
	for _, leaf := range c.Fleet.AllMachines() {
		err := leaf.Machine.InitSSH(c.Flags.LocalMachineHostname, *c.Nix)
		if err != nil {
			return errors.Wrap(err, "failed to init machine SSH")
		}
	}

	return nil
}
