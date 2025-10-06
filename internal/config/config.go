package config

import (
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/sorted_map"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"go.uber.org/atomic"
)

const (
	ContextConfigKey = "config"
)

type Config struct {
	Flags *config_flags.Flags `yaml:"flags"`
	Root  *Root               `yaml:"root"`
	Tui   *Tui                `yaml:"tui"`
}

// Root

type Root struct {
	Flakes                       sorted_map.SortedMap[string, *Flake] `yaml:"flakes"`
	config_attributes.Attributes `yaml:",squash"`
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
	config_attributes.Attributes `yaml:",squash"`
	Url                          string     `yaml:"url"` // Flake path (eg. `path:...`) or url (eg. `ssh:...`)
	FlakeHooks                   FlakeHooks `yaml:"flakeHooks"`
}

type FlakeHooks struct {
	Pre  string `yaml:",pre"`
	Post string `yaml:",post"`
}

func (f *Flake) Init(name string, passAttr *config_attributes.Attributes, flags *config_flags.Flags) error {
	err := f.Attributes.Init(name, passAttr, flags)
	if err != nil {
		return err
	}

	return nil
}

// Configuration

type Configuration struct {
	Machines                     sorted_map.SortedMap[string, *Machine] `yaml:"machines"`
	config_attributes.Attributes `yaml:",squash"`
	// TODO: FlakeOutput string `yaml:"flakeOutput"` // Option to override if non-standard style
	// Internal
	Flake     *Flake
	MetaBuild *MetaBuild
}

type MetaBuild struct {
	SystemClosure string
	DiskoScript   string // Only on bootstrap
}

func (c *Configuration) Init(name string, parent *Flake, flags *config_flags.Flags) error {
	err := c.Attributes.Init(name, &parent.Attributes, flags)
	if err != nil {
		return err
	}

	// Internal

	c.Flake = parent

	if c.MetaBuild == nil {
		c.MetaBuild = &MetaBuild{}
	}

	return nil
}

// Machine

type Machine struct {
	config_attributes.Attributes `yaml:",squash"`
	// Internal
	Configuration *Configuration
	MetaStatus    *MetaStatus
}

type MetaStatus struct { // Atomic due to being read and write at the same time
	Reachable      atomic.Bool
	SSHConnectable atomic.Bool
	Architecture   atomic.String
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

	if m.SudoProgram == nil {
		sudoProgram := "sudo" // Default sudo program
		m.SudoProgram = &sudoProgram
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

// Tui

type Tui struct {
	ShowAllBuildLogs bool `yaml:"showAllBuildLogs"`
	ColorScheme      *ColorScheme
}
