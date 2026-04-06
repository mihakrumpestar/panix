package config

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/gookit/goutil/dump"
	config_attributes "github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

var ErrNoFlakesAfterFilter = errors.New("no flakes left after filtering")

type NixArgs map[string]string

func LoadConfig(parsedFlags flags.Flags, commandPhases []phases.Phase) (*Config, error) {
	dump.Config(func(d *dump.Options) {
		d.BytesAsString = true
		d.SkipNilField = true
		d.MaxDepth = 99
	})

	nixArgs := map[string]string{
		"env":           parsedFlags.Env,
		"validatePaths": strconv.FormatBool(!parsedFlags.NoValidatePaths),
	}

	for k, v := range parsedFlags.NixArgs {
		nixArgs[k] = v
	}

	conf, err := LoadNixConfig(parsedFlags.Config, nixArgs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load nix config")
	}

	err = applyConfigDefaults(conf, parsedFlags)
	if err != nil {
		return nil, err
	}

	err = validateAndInitConfig(conf)
	if err != nil {
		return nil, err
	}

	err = conf.filterRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to filter config")
	}

	conf.Phases, err = phases.ValidatePhases(commandPhases, conf.Flags.SkipPhases)
	if err != nil {
		return nil, errors.Wrap(err, "invalid phases")
	}

	if conf.Flags.Logging.Debug {
		dump.P(conf.Flags)
		dump.P(conf.Flakes)
	}

	return conf, nil
}

func LoadNixConfig(configPath string, args NixArgs) (*Config, error) {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get absolute path")
	}

	cmdArgs := []string{"eval", "--json", "--impure"}

	// Build expression that:
	// 1. Imports user's panix.nix (which already includes panix-options.nix via imports)
	// 2. Evaluates the module system
	// 3. Transforms attrsets to ordered lists with name fields
	importExpr := fmt.Sprintf(`
let
  lib = import <nixpkgs/lib>;
  
  # Sort attrset by source line position
  sortByLine = attrs:
    let
      itemsWithPos = builtins.mapAttrs (name: value: {
        inherit name value;
        pos = builtins.unsafeGetAttrPos name attrs;
      }) attrs;
      items = builtins.attrValues itemsWithPos;
      sorted = builtins.sort (a: b:
        if a.pos == null then false
        else if b.pos == null then true
        else a.pos.line < b.pos.line
      ) items;
    in map (item: item.value // { inherit (item) name; }) sorted;
  
  # Transform flakes attrset to ordered list with names
  transformFlakes = flakes:
    map (flake: flake // {
      configurations = map (config: config // {
        machines = sortByLine config.machines;
      }) (sortByLine flake.configurations);
    }) (sortByLine flakes);
  
  # Evaluate the user's config file (which imports panix-options.nix)
  evaluated = lib.evalModules {
    modules = [ 
      { _module.args = { inherit lib; }; }
      (import %s { 
        env = %s; 
        validatePaths = %s; 
      })
    ];
  };
in {
  flags = evaluated.config.flags;
  flakes = transformFlakes evaluated.config.flakes;
}
`,
		strconv.Quote(absPath),
		strconv.Quote(args["env"]),
		args["validatePaths"])

	cmdArgs = append(cmdArgs, "--expr", importExpr)

	output, err := exec.Command("nix", cmdArgs...).Output()
	if err != nil {
		return nil, parseNixError(err)
	}

	var config Config
	if err := json.Unmarshal(output, &config); err != nil {
		return nil, errors.Wrap(err, "failed to parse nix output")
	}

	return &config, nil
}

func parseNixError(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := string(exitErr.Stderr)
		return errors.New(stderr)
	}
	return errors.Wrap(err, "nix eval failed")
}

func applyConfigDefaults(conf *Config, parsedFlags flags.Flags) error {
	if conf.Flags == nil {
		conf.Flags = &flags.Flags{}
	}

	err := conf.Flags.MergeConfWithCliFlags(parsedFlags)
	if err != nil {
		return errors.Wrap(err, "failed merging config with cli flags")
	}

	if conf.ColorScheme == nil {
		conf.ColorScheme = defaultColorScheme()
	}

	err = flags.InitLogging(conf.Flags.Logging)
	if err != nil {
		return errors.Wrap(err, "failed to initialize logging")
	}

	return nil
}

func validateAndInitConfig(conf *Config) error {
	err := conf.ValidateStructTags()
	if err != nil {
		return errors.Wrap(err, "invalid configuration")
	}

	err = conf.initializeFlakes()
	if err != nil {
		return errors.Wrap(err, "invalid configuration")
	}

	return nil
}

func (c *Config) InitializeFlakes() error {
	for _, flake := range c.Flakes {
		err := flake.Init(flake.Name, &config_attributes.Attributes{Flags: c.Flags})
		if err != nil {
			return err
		}

		for _, configuration := range flake.Configurations {
			err := configuration.Init(configuration.Name, flake)
			if err != nil {
				return err
			}

			for _, machine := range configuration.Machines {
				err = machine.Init(machine.Name, configuration)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *Config) initializeFlakes() error {
	for _, flake := range c.Flakes {
		err := flake.Init(flake.Name, &config_attributes.Attributes{Flags: c.Flags})
		if err != nil {
			return err
		}

		for _, configuration := range flake.Configurations {
			err := configuration.Init(configuration.Name, flake)
			if err != nil {
				return err
			}

			for _, machine := range configuration.Machines {
				err = machine.Init(machine.Name, configuration)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *Config) filterRoot() error {
	var activeFlakes []*Flake
	for _, flake := range c.Flakes {
		if flake != nil && !flake.Disabled && flake.Configurations != nil && len(flake.Configurations) > 0 {
			c.filterFlakeConfigurations(flake)
			if len(flake.Configurations) > 0 {
				activeFlakes = append(activeFlakes, flake)
			}
		}
	}
	c.Flakes = activeFlakes

	if len(c.Flakes) == 0 {
		return ErrNoFlakesAfterFilter
	}

	return nil
}

func (c *Config) filterFlakeConfigurations(flake *Flake) {
	var activeConfigs []*Configuration
	for _, config := range flake.Configurations {
		if config != nil && !config.Disabled && config.Machines != nil && len(config.Machines) > 0 {
			c.filterConfigurationMachines(config)
			if len(config.Machines) > 0 {
				activeConfigs = append(activeConfigs, config)
			}
		}
	}
	flake.Configurations = activeConfigs
}

func (c *Config) filterConfigurationMachines(config *Configuration) {
	var activeMachines []*Machine
	for _, machine := range config.Machines {
		if machine != nil && !machine.Disabled && machineContainsTags(machine.Tags, c.Flags.Tags) {
			activeMachines = append(activeMachines, machine)
		} else {
			log.Debug().
				Bool("machine == nil", machine == nil).
				Bool("disabled", machine != nil && machine.Disabled).
				Strs("machine.Tags", machine.Tags).
				Msgf("filtering out machine")
		}
	}
	config.Machines = activeMachines
}

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
