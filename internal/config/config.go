package config

import (
	config_attributes "github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
)

type Config struct {
	Flags       *flags.Flags `yaml:"flags"`
	Root        *Root        `yaml:"root,required" validate:"required"`
	ColorScheme *ColorScheme `yaml:"-" validate:"-"`

	// Internal
	Phases []phases.Phase `yaml:"-" validate:"-"`
}

// Root

type Root struct {
	Flakes                       *OrderedMap[string, *Flake] `yaml:"flakes,required"`
	config_attributes.Attributes `yaml:",inline"`
}

func (r *Root) Init(f *flags.Flags) error {
	return errors.Wrap(
		r.Attributes.Init("root", &config_attributes.Attributes{Flags: f}, false),
		"failed to initialize root attributes",
	)
}

// Flake

type Flake struct {
	Configurations               *OrderedMap[string, *Configuration] `yaml:"configurations,required" validate:"required"`
	config_attributes.Attributes `yaml:",inline"`
	URL                          string `yaml:"url,required" validate:"required,uri" desc:"Flake path (eg. 'path:...') or url (eg. 'ssh:...' 'github:...'), reference https://nix.dev/manual/nix/2.33/command-ref/new-cli/nix3-flake.html#url-like-syntax"` //nolint:lll
}

func (f *Flake) Init(name string, attr *config_attributes.Attributes) error {
	return errors.Wrap(f.Attributes.Init(name, attr, false), "failed to initialize flake")
}

// Configuration

type Configuration struct {
	Machines                     *OrderedMap[string, *Machine] `yaml:"machines,required" validate:"required"`
	config_attributes.Attributes `yaml:",inline"`
	FlakeOutput                  string `yaml:"flake_output" desc:"Override flake output (default: nixosConfigurations.<name>.config.system.build.toplevel)"` //nolint:lll
	// Internal
	ParentFlake *Flake     `yaml:"-" validate:"-"`
	MetaBuild   *MetaBuild `yaml:"-" validate:"-"`
}

type MetaBuild struct {
	SystemClosure string
}

func (c *Configuration) Init(name string, parent *Flake) error {
	c.ParentFlake = parent

	err := c.Attributes.Init(name, &parent.Attributes, false)
	if err != nil {
		return errors.Wrap(err, "failed to init configuration attributes")
	}

	return nil
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
	RequiresKexec  atomic.Bool
	Generation     atomic.Uint32
	Date           atomic.String
	Nixos          atomic.String
	Kernel         atomic.String
	activeSSH      atomic.Pointer[ssh.SSHClient]
}

func (m *MetaInspect) GetActiveSSH() *ssh.SSHClient {
	return m.activeSSH.Load()
}

func (m *MetaInspect) SetActiveSSH(sshClient *ssh.SSHClient) {
	m.activeSSH.Store(sshClient)
}

func (m *Machine) Init(name string, parent *Configuration) error {
	err := m.Attributes.Init(name, &parent.Attributes, true)
	if err != nil {
		return errors.Wrap(err, "failed to initialize machine")
	}

	m.ParentConfiguration = parent
	m.MetaInspect = &MetaInspect{}
	m.MetaInspect.SetActiveSSH(m.SSH)

	return nil
}

var (
	emptySudo = []string{}
	sudoCmd   = []string{"sudo"}
)

func (m *Machine) MaybeSudo() []string {
	if m.MetaInspect.IsRoot.Load() {
		return emptySudo
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

func (m *Machine) SwitchToBootstrapSSH() {
	if m.Bootstrap.SSH != nil {
		m.MetaInspect.SetActiveSSH(m.Bootstrap.SSH)
	}
}

func (m *Machine) SwitchToRegularSSH() {
	m.MetaInspect.SetActiveSSH(m.SSH)
}
