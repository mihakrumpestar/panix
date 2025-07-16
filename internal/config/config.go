package config

import (
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/gobeam/stringy"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
	"github.com/spf13/pflag"
	"github.com/yassinebenaid/godump"
)

type Config struct {
	Global Global            `koanf:"global"`
	Flakes map[string]*Flake `koanf:"flakes"`
}

type Global struct {
	Filters           Filters                             `koanf:"filters"`
	RequireAllSuccess bool                                `koanf:"requireAllSuccess"`
	AutoBootstrap     bool                                `koanf:"autoBootstrap"`
	LocalMachine      string                              `koanf:"localMachine"`
	DryRun            bool                                `koanf:"dryRun"`
	Ssh               *Ssh                                `koanf:"ssh"`
	Timeout           time.Duration                       `koanf:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int                                 `koanf:"concurrency"`
	SkipPhases        []workflow_definition.WorkflowPhase `koanf:"skipPhases"`
	Verbose           bool                                `koanf:"verbose"`
	Debug             bool                                `koanf:"debug"`
	//Json              bool                                `koanf:"json"` // Maybe later
}

type Filters struct {
	Flakes         []string `koanf:"flakes"`
	Configurations []string `koanf:"configurations"`
	Machines       []string `koanf:"machines"`
	Tags           []string `koanf:"tags"`
}

type Flake struct {
	Url             string `koanf:"url"` // Flake path or url
	treeStyleParams `koanf:",squash"`
	Configurations  map[string]*Configuration `koanf:"configurations"`
}

type Configuration struct {
	FlakeOutput     string `koanf:"flakeOutput"` // Override if not standard style
	treeStyleParams `koanf:",squash"`
	Machines        map[url.URL]*Machine `koanf:"machines"` // Key here is the ssh URL: alias, user@host or user@host:port
}

type Machine struct {
	treeStyleParams `koanf:",squash"`
}

type treeStyleParams struct {
	Ssh      *Ssh           `koanf:"ssh,omitempty"`
	Tags     []string       `koanf:"tags"`
	Secrets  []SecretConfig `koanf:"secrets,omitempty"`
	Disabled bool           `koanf:"disabled"`
}

type Ssh struct {
	Url        *url.URL `koanf:"-"`
	PrivateKey string   `koanf:"privateKey"`
	PublicKey  string   `koanf:"publicKey"`
}

type SecretConfig struct {
	LocalPath  string `koanf:"localPath"`
	RemotePath string `koanf:"remotePath"`
	Mode       string `koanf:"mode"`
}

func urlURLHookFunc() func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		// Check that the data is string
		if f.Kind() != reflect.String {
			return data, nil
		}

		// Check that the target type is our custom type
		if t != reflect.TypeOf(url.URL{}) {
			return data, nil
		}

		// Return the parsed value
		dataVal := data.(string)
		url, err := url.Parse(dataVal)
		if err != nil {
			return nil, err
		}
		return *url, nil
	}
}

var (
	C Config
)

// LoadConfig reads and parses the config file.
func LoadConfig(configFile string, flags *pflag.FlagSet) (*Config, error) {
	k := koanf.New(".")

	// Load YAML config file
	err := k.Load(file.Provider(configFile), yaml.Parser())
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %w", err)
	}

	// Load environment variables with PANIX_ prefix
	err = k.Load(
		env.Provider(
			"PANIX_",
			".",
			func(s string) string {
				// Convert PANIX_SOME_KEY to some.key
				return strings.ToLower(strings.TrimPrefix(s, "PANIX_"))
			},
		),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Load command line flags if provided
	if flags != nil {
		err := k.Load(
			posflag.ProviderWithFlag(
				flags,
				".",
				k,
				func(f *pflag.Flag) (string, interface{}) {
					// Transform the key in whatever manner.

					keyRaw := stringy.New(f.Name).CamelCase()
					// If special nested "filters"
					if slices.Contains([]string{"flakes", "configurations", "machines"}, keyRaw.Get()) {
						keyRaw.Prefix("filters.")
					}

					key := keyRaw.Prefix("global.")

					//fmt.Println(key)

					// Use FlagVal() and then transform the value, or don't use it at all
					// and add custom logic to parse the value.
					val := posflag.FlagVal(flags, f)

					return key, val
				},
			),
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("error loading command line flags: %w", err)
		}
	}

	// Unmarshal the configuration
	if err := k.UnmarshalWithConf("", &C, koanf.UnmarshalConf{
		Tag: "koanf",
		DecoderConfig: &mapstructure.DecoderConfig{
			DecodeHook:       urlURLHookFunc(),
			WeaklyTypedInput: true,
			Result:           &C,
		},
	}); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	// Convert timeout from miliseconds to seconds for duration
	C.Global.Timeout *= time.Second

	if C.Global.Debug {
		godump.Dump(C)
	}

	err = C.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	C.Flakes, err = C.filterConfigEntrys()
	if err != nil {
		return nil, fmt.Errorf("failed to filter config: %w", err)
	}

	err = C.validateConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if C.Global.Debug {
		godump.Dump(C)
	}

	return &C, nil
}

func (c *Config) validateConfig() error {
	if len(c.Flakes) == 0 {
		return fmt.Errorf("flakes is required")
	}

	for flakeName, flake := range c.Flakes {
		if len(flake.Configurations) == 0 {
			return fmt.Errorf("flakes[%s]configurations is empty", flakeName)
		}

		for configurationName, configuration := range flake.Configurations {
			if len(configuration.Machines) == 0 {
				return fmt.Errorf("flakes[%s]configurations[%s]machines is empty", flakeName, configurationName)
			}

			for machineName := range configuration.Machines {
				if machineName.Host == "" {
					return fmt.Errorf("flakes[%s]configurations[%s]machines[%s].ssh host is empty", flakeName, configurationName, machineName.String())
				}
			}
		}
	}

	return nil
}

// FilterConfigEntrys filters the configuration based on command-line or global selections.
// An entry is kept if it matches all provided filters (flakes, configurations, machines, tags) and is not disabled.
// If a filter type is not provided (e.g., the 'machines' slice is empty), it is not used for filtering.
func (c *Config) filterConfigEntrys() (map[string]*Flake, error) {
	cC := *c // Copy

	sc, err := LoadSshConfig()
	if err != nil {
		return nil, err
	}

	flakesFilter := cC.Global.Filters.Flakes
	configurationsFilter := cC.Global.Filters.Configurations
	machinesFilter := cC.Global.Filters.Machines

	for flakeName, flake := range cC.Flakes {
		if (len(flakesFilter) > 0 && !slices.Contains(flakesFilter, flakeName)) || flake.Disabled {
			delete(cC.Flakes, flakeName)
			continue
		}

		for configurationName, configuration := range flake.Configurations {
			if (len(configurationsFilter) > 0 && !slices.Contains(configurationsFilter, configurationName)) || configuration.Disabled {
				delete(flake.Configurations, configurationName)
				continue
			}

			for machineName, machine := range configuration.Machines {
				if machine == nil {
					machine = &Machine{}
				}

				if (len(machinesFilter) > 0 && !slices.Contains(machinesFilter, machineName.String())) || machine.Disabled {
					delete(configuration.Machines, machineName)
					continue
				}

				// A machine must match the tag filters if they are provided.
				if len(cC.Global.Filters.Tags) > 0 {
					allMachineTags := flake.Tags
					allMachineTags = append(allMachineTags, configuration.Tags...)
					allMachineTags = append(allMachineTags, machine.Tags...)

					if !matchesTags(allMachineTags, C.Global.Filters.Tags) {
						delete(configuration.Machines, machineName)
						continue
					}
				}

				if machineName.User.Username() == "" {
					if machine.Ssh == nil {
						machine.Ssh = &Ssh{}
					}
					machine.Ssh.Url, err = sc.RetriveFullParamsFromSshConfig(machineName)
					if err != nil {
						return nil, err
					}
				}
			}

			if len(configuration.Machines) == 0 {
				delete(flake.Configurations, configurationName)
			}
		}

		if len(flake.Configurations) == 0 {
			delete(cC.Flakes, flakeName)
		}
	}

	if len(cC.Flakes) == 0 {
		return nil, fmt.Errorf("flakes configuration empty after filtering")
	}

	return cC.Flakes, nil
}

func matchesTags(tags, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}

	for _, filterTag := range filterTags {
		if !slices.Contains(tags, filterTag) {
			return false
		}
	}

	return true
}
