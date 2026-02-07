package config

import (
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/sorted_map"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"go.uber.org/atomic"
)

const (
	ContextConfigKey = "config"
)

type Config struct {
	Flags       *config_flags.Flags `yaml:"flags"`
	Root        *Root               `yaml:"root"`
	ColorScheme *ColorScheme        `yaml:"colorScheme"`

	// Internal
	*logs.TargetsLogs
}

// Root

type Root struct {
	Flakes                       sorted_map.SortedMap[string, *Flake] `yaml:"flakes"`
	config_attributes.Attributes `yaml:",inline"`
}

func (r *Root) Init() error {
	err := r.Attributes.Init("", &config_attributes.Attributes{}, &config_flags.Flags{})
	if err != nil {
		return err
	}

	return nil
}

// Flake

type Flake struct {
	Configurations               sorted_map.SortedMap[string, *Configuration] `yaml:"configurations"`
	config_attributes.Attributes `yaml:",inline"`
	Url                          string     `yaml:"url"` // Flake path (eg. `path:...`) or url (eg. `ssh:...`)
	FlakeHooks                   FlakeHooks `yaml:"flakeHooks"`
}

type FlakeHooks struct {
	Pre  string `yaml:"pre"`
	Post string `yaml:"post"`
}

func (f *Flake) Init(name string, attr *config_attributes.Attributes, flags *config_flags.Flags) error {
	err := f.Attributes.Init(name, attr, flags)
	if err != nil {
		return err
	}

	return nil
}

// Configuration

type Configuration struct {
	Machines                     sorted_map.SortedMap[string, *Machine] `yaml:"machines"`
	config_attributes.Attributes `yaml:",inline"`
	// TODO: FlakeOutput string `yaml:"flakeOutput"` // Option to override if non-standard style
	// Internal
	Flake     *Flake
	MetaBuild MetaBuild
}

type MetaBuild struct {
	SystemClosure string
}

func (c *Configuration) Init(name string, parent *Flake, flags *config_flags.Flags) error {
	err := c.Attributes.Init(name, &parent.Attributes, flags)
	if err != nil {
		return err
	}

	// Internal

	c.Flake = parent

	return nil
}

// Machine

type Machine struct {
	config_attributes.Attributes `yaml:",inline"`
	// Internal
	Configuration *Configuration
	MetaStatus    *MetaStatus
}

type MetaStatus struct { // Atomic due to being read and write at the same time
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

	if m.Ssh == nil { // Has to be initialized before InitAttributes
		m.Ssh = &ssh.SshClient{}
	}

	// Regular

	err := m.Attributes.Init(name, &parent.Attributes, flags)
	if err != nil {
		return err
	}

	// Internal

	m.Configuration = parent

	if m.MetaStatus == nil {
		m.MetaStatus = &MetaStatus{}
	}

	return nil
}

func (m *Machine) MaybeSudo() []string {
	if m.MetaStatus.IsRoot.Load() {
		return make([]string, 0) // Return zero slice (instead of nil), since this might be the starting slice
	}

	if m.OverrideSudoProgram == "" {
		return []string{"sudo"}
	}

	return []string{m.OverrideSudoProgram}
}

func (m *Machine) MaybeBootstrapingPath(restOfPath string) string {
	if m.Flags.Bootstrap.DisableAuto || !m.MetaStatus.Bootstrapped.Load() {
		return restOfPath
	}

	return "/mnt" + restOfPath
}
