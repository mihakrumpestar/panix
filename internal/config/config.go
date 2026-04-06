package config

import (
	"github.com/kirill-scherba/omap"
	config_attributes "github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
)

type Config struct {
	Flags       *flags.Flags `json:"flags"`
	Flakes      []*Flake     `json:"flakes" validate:"required,dive"`
	ColorScheme *ColorScheme `json:"-" validate:"-"`

	Phases             []phases.Phase `json:"-" validate:"-"`
	RollbackGeneration int            `json:"-" validate:"-"`
}

type Flake struct {
	Name                         string           `json:"name"`
	URL                          string           `json:"url" validate:"required"`
	Configurations               []*Configuration `json:"configurations,omitempty" validate:"dive"`
	config_attributes.Attributes `json:",inline"`
}

func (f *Flake) Init(name string, attr *config_attributes.Attributes) error {
	f.Name = name
	return errors.Wrap(f.Attributes.Init(name, attr, false), "failed to initialize flake")
}

type Configuration struct {
	Name                         string     `json:"name"`
	Machines                     []*Machine `json:"machines" validate:"required,dive"`
	FlakeOutput                  string     `json:"flake_output,omitempty"`
	config_attributes.Attributes `json:",inline"`
	ParentFlake                  *Flake     `json:"-" validate:"-"`
	MetaBuild                    *MetaBuild `json:"-" validate:"-"`
}

type MetaBuild struct {
	SystemClosure string
}

func (c *Configuration) Init(name string, parent *Flake) error {
	c.Name = name
	c.ParentFlake = parent

	err := c.Attributes.Init(name, &parent.Attributes, false)
	if err != nil {
		return errors.Wrap(err, "failed to init configuration attributes")
	}

	return nil
}

type Machine struct {
	Name                         string `json:"name"`
	config_attributes.Attributes `json:",inline"`
	ParentConfiguration          *Configuration `json:"-" validate:"-"`
	MetaInspect                  *MetaInspect   `json:"-" validate:"-"`
}

type MetaInspect struct {
	Reachable      atomic.Bool
	SSHConnectable atomic.Bool
	Architecture   atomic.String
	IsRoot         atomic.Bool
	Bootstrapped   atomic.Bool
	RequiresKexec  atomic.Bool
	Generations    atomic.Pointer[GenerationsData]

	activeSSH atomic.Pointer[ssh.SSHClient]
}

type GenerationsData struct {
	Current     uint
	Generations *omap.Omap[uint, *GenerationInfo]
}

type GenerationInfo struct {
	Date    string
	Nixos   string
	Kernel  string
	Current bool
}

func (m *MetaInspect) GetActiveSSH() *ssh.SSHClient {
	return m.activeSSH.Load()
}

func (m *MetaInspect) SetActiveSSH(sshClient *ssh.SSHClient) {
	m.activeSSH.Store(sshClient)
}

func (m *Machine) Init(name string, parent *Configuration) error {
	m.Name = name
	err := m.Attributes.Init(name, &parent.Attributes, true)
	if err != nil {
		return errors.Wrap(err, "failed to initialize machine")
	}

	m.ParentConfiguration = parent
	m.MetaInspect = &MetaInspect{}
	m.MetaInspect.SetActiveSSH(m.SSH)

	if m.Flags.ActivationMode != flags.ActivationModeSwitch {
		m.ActivationMode = m.Flags.ActivationMode
	}

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

func (m *Machine) GetKexecSSH() *ssh.SSHClient {
	activeSSH := m.MetaInspect.GetActiveSSH()

	port := ssh.DefaultSSHPort
	if m.Bootstrap.Kexec != nil && m.Bootstrap.Kexec.SSHPort != 0 {
		port = m.Bootstrap.Kexec.SSHPort
	}

	return &ssh.SSHClient{
		Hostname:              activeSSH.Hostname,
		Port:                  port,
		Username:              activeSSH.Username,
		IdentityFile:          activeSSH.IdentityFile,
		StrictKeyChecking:     false,
		DisableAutoAddHostKey: true,
		IsLocal:               activeSSH.IsLocal,
	}
}
