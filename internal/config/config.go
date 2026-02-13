package config

import (
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"go.uber.org/atomic"
)

type Config struct {
	Flags       *config_flags.Flags `yaml:"flags"`
	Root        *Root               `yaml:"root"`
	ColorScheme *ColorScheme        `yaml:"color_scheme"`

	// Internal
	*logs.TargetsLogs `yaml:"-"`
}

// Root

type Root struct {
	Flakes                       *OrderedMap[string, *Flake] `yaml:"flakes"`
	config_attributes.Attributes `yaml:",inline"`
}

func (r *Root) Init() error {
	return r.Attributes.Init("", &config_attributes.Attributes{}, &config_flags.Flags{})
}

// Flake

type Flake struct {
	Configurations               *OrderedMap[string, *Configuration] `yaml:"configurations"`
	config_attributes.Attributes `yaml:",inline"`
	URL                          string     `yaml:"url"` // Flake path (eg. `path:...`) or url (eg. `ssh:...`)
	FlakeHooks                   FlakeHooks `yaml:"flake_hooks"`
}

type FlakeHooks struct {
	Pre  string `yaml:"pre"`
	Post string `yaml:"post"`
}

func (f *Flake) Init(name string, attr *config_attributes.Attributes, flags *config_flags.Flags) error {
	return f.Attributes.Init(name, attr, flags)
}

// Configuration

type Configuration struct {
	Machines                     *OrderedMap[string, *Machine] `yaml:"machines"`
	config_attributes.Attributes `yaml:",inline"`
	// TODO: FlakeOutput string `yaml:"flake_output"` // Option to override if non-standard style
	// Internal
	ParentFlake *Flake     `yaml:"-"`
	MetaBuild   *MetaBuild `yaml:"-"`
}

type MetaBuild struct {
	SystemClosure string
}

func (c *Configuration) Init(name string, parent *Flake, flags *config_flags.Flags) error {
	c.ParentFlake = parent
	return c.Attributes.Init(name, &parent.Attributes, flags)
}

// Machine

type Machine struct {
	config_attributes.Attributes `yaml:",inline"`
	// Internal
	ParentConfiguration *Configuration `yaml:"-"`
	MetaInspect         *MetaInspect   `yaml:"-"`
}

type MetaInspect struct { // Atomic due to being read and write at the same time
	Reachable      atomic.Bool
	SSHConnectable atomic.Bool
	Architecture   atomic.String
	IsRoot         atomic.Bool
	Bootstrapped   atomic.Bool
	Generation     atomic.Uint32
	Date           atomic.String
	Nixos          atomic.String
	Kernel         atomic.String
}

func (m *Machine) Init(name string, parent *Configuration, flags *config_flags.Flags) error {
	// Only machine has them always initialized (root, flake, configurations do not)

	if m.SSH == nil { // Has to be initialized before InitAttributes
		m.SSH = &ssh.SshClient{}
	}

	// Regular

	err := m.Attributes.Init(name, &parent.Attributes, flags)
	if err != nil {
		return err
	}

	// Internal

	m.ParentConfiguration = parent
	m.MetaInspect = &MetaInspect{}

	return nil
}

var (
	emptySudo = []string{}
	sudoCmd   = []string{"sudo"}
)

func (m *Machine) MaybeSudo() []string {
	if m.MetaInspect.IsRoot.Load() {
		return emptySudo // Return zero slice (instead of nil), since this might be the starting slice
	}

	if m.OverrideSudoProgram == "" {
		return sudoCmd
	}

	return []string{m.OverrideSudoProgram}
}

func (m *Machine) MaybeBootstrappingPath(restOfPath string) string {
	if m.Flags.Bootstrap.DisableAuto || m.MetaInspect.Bootstrapped.Load() {
		return restOfPath
	}

	return "/mnt" + restOfPath
}
