package config

import (
	"bytes"
	"os"
	"strconv"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/gookit/goutil/dump"
	"github.com/mihakrumpestar/panix/gen"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/template"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

func LoadConfig(parsedFlags flags.Flags, commandPhases []phases.Phase) (*Config, error) {
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
	err = conf.Fleet.Filter()
	if err != nil {
		return nil, errors.Wrap(err, "failed to filter config")
	}

	conf.Phases, err = phases.ValidatePhases(commandPhases, conf.Flags.SkipPhases)
	if err != nil {
		return nil, errors.Wrap(err, "invalid phases")
	}

	conf.filterUnusedPhases()

	if conf.Flags.Logging.Debug {
		dump.P(conf.Flags)
	}

	conf.StartTime = time.Now()
	conf.PanixVersion = gen.Version()

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
	decoder := yaml.NewDecoder(bytes.NewReader(processedYAML))

	err = decoder.Decode(conf)
	if err != nil {
		return nil, errors.New(yaml.FormatError(err, true, false))
	}

	return conf, nil
}

// applyConfigDefaults merges configuration with CLI flags and applies defaults.
func applyConfigDefaults(conf *Config, parsedFlags flags.Flags) error {
	// Apply defaults
	if conf.Flags == nil {
		conf.Flags = &flags.Flags{}
	}

	err := conf.Flags.MergeConfWithCliFlags(parsedFlags)
	if err != nil {
		return errors.Wrap(err, "failed merging config with cli flags")
	}

	if conf.ColorScheme == nil {
		conf.ColorScheme = DefaultColorScheme()
	}

	conf.Flags.DefautlIfNoTTY()

	err = logger.InitLogging(conf.Flags.Logging, conf.Flags.Output)
	if err != nil {
		return errors.Wrap(err, "failed to initialize logging")
	}

	return nil
}

func (c *Config) initFleet() error {
	err := c.Fleet.Init(c.Flags)
	if err != nil {
		return err
	}

	for _, flake := range c.Fleet.Flakes.Pairs() {
		err = flake.Value.Init(flake.Key, &c.Fleet.Attributes)
		if err != nil {
			return err
		}

		for _, cfg := range flake.Value.Configurations.Pairs() {
			err = cfg.Value.Init(cfg.Key, &flake.Value.Attributes)
			if err != nil {
				return err
			}

			for _, machine := range cfg.Value.Machines.Pairs() {
				err = machine.Value.Init(machine.Key, &cfg.Value.Attributes)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
