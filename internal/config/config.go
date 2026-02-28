package config

import (
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"go.uber.org/atomic"
)

type Config struct {
	Flags       *config_flags.Flags `yaml:"flags"`
	Root        *Root               `yaml:"root,required" validate:"required"`
	ColorScheme *ColorScheme        `yaml:"-" validate:"-"`

	// Internal
	Phases []phases.Phase `yaml:"-" validate:"-"`
}

// Root

type Root struct {
	Flakes                       *OrderedMap[string, *Flake] `yaml:"flakes,required"`
	config_attributes.Attributes `yaml:",inline"`
}

func (r *Root) Init(flags *config_flags.Flags) error {
	return r.Attributes.Init("root", &config_attributes.Attributes{Flags: flags}, false)
}

// Flake

type Flake struct {
	Configurations               *OrderedMap[string, *Configuration] `yaml:"configurations,required" validate:"required"`
	config_attributes.Attributes `yaml:",inline"`
	URL                          string `yaml:"url,required" validate:"required,uri" desc:"Flake path (eg. 'path:...') or url (eg. 'ssh:...'), reference https://nix.dev/manual/nix/2.33/command-ref/new-cli/nix3-flake.html#url-like-syntax"`
}

func (f *Flake) Init(name string, attr *config_attributes.Attributes) error {
	return f.Attributes.Init(name, attr, false)
}

// Configuration

type Configuration struct {
	Machines                     *OrderedMap[string, *Machine] `yaml:"machines,required" validate:"required"`
	config_attributes.Attributes `yaml:",inline"`
	FlakeOutput                  string `yaml:"flake_output" desc:"Option to override flake output if non-standard style (default: nixosConfigurations.<name>.config.system.build.toplevel)"`
	// Internal
	ParentFlake *Flake     `yaml:"-" validate:"-"`
	MetaBuild   *MetaBuild `yaml:"-" validate:"-"`
}

type MetaBuild struct {
	SystemClosure string
}

func (c *Configuration) Init(name string, parent *Flake) error {
	c.ParentFlake = parent
	return c.Attributes.Init(name, &parent.Attributes, false)
}

// Machine

type Machine struct {
	config_attributes.Attributes `yaml:",inline"`
	ParentConfiguration          *Configuration `yaml:"-" validate:"-"`
	MetaInspect                  *MetaInspect   `yaml:"-" validate:"-"`
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

func (m *Machine) Init(name string, parent *Configuration) error {
	err := m.Attributes.Init(name, &parent.Attributes, true)
	if err != nil {
		return err
	}

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
