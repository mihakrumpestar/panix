package machine

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/jsonerror"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

type SSHType string

const (
	SSHTypeBootstrap SSHType = "bootstrap"
	SSHTypeKexec     SSHType = "kexec"
	SSHTypeRegular   SSHType = "regular"
)

//nolint:lll
type Machine struct {
	attributes.Attributes `yaml:",inline"`

	// Internal
	MetaInspect *atomicpointer.AtomicPointer[MetaInspect] `yaml:"-" json:"meta_inspect,omitempty"` // Needs to be atomic (updates with executioner)
	Logs        *logs.Logs                                `yaml:"-" json:"logs,omitempty"`
	State       *atomicpointer.AtomicPointer[State]       `yaml:"-" json:"machine_state,omitempty"` // Needs to be atomic (updates from both workflow goroutines and UI)
}

type MetaInspect struct {
	Reachable      bool `yaml:"-" json:"reachable,omitempty"`
	SSHConnectable bool `yaml:"-" json:"ssh_connectable,omitempty"`
	IsRoot         bool `yaml:"-" json:"is_root,omitempty"`
	Bootstrapped   bool `yaml:"-" json:"bootstrapped,omitempty"`
	RequiresKexec  bool `yaml:"-" json:"requires_kexec,omitempty"`

	Architecture string       `yaml:"-" json:"architecture,omitempty"`
	Generations  *Generations `yaml:"-" json:"generations,omitempty"`
	Date         string       `yaml:"-" json:"date,omitempty"`
	Nixos        string       `yaml:"-" json:"nixos,omitempty"`
	Kernel       string       `yaml:"-" json:"kernel,omitempty"`
}

type State struct {
	Status    stats.StatsState     `yaml:"-" json:"status"`
	StatusMsg string               `yaml:"-" json:"status_msg"`
	Phase     phase.Phase          `yaml:"-" json:"phase"`
	Error     *jsonerror.JSONError `yaml:"-" json:"error,omitempty"`
	ActiveSSH SSHType              `yaml:"-" json:"active_ssh,omitempty" default:"regular"`
}

type Generations struct {
	Current   uint   `yaml:"-" json:"current"`
	Available []uint `yaml:"-" json:"available"`
}

func (m *Machine) PostUnmarshalInit(name string, parentAttr *attributes.Attributes) {
	if m.Logs == nil {
		m.Logs = logs.New()
	} else {
		m.Logs.PostUnmarshalInit()
	}

	if m.State == nil {
		m.State = atomicpointer.New[State]()
	}
}

func (m *Machine) Init(name string, parentAttributes *attributes.Attributes, localMachineHostname string) error {
	err := m.Attributes.Init(name, parentAttributes, true, localMachineHostname)
	if err != nil {
		return errors.Wrap(err, "failed to initialize machine")
	}

	m.MetaInspect = atomicpointer.New[MetaInspect]()
	m.State = atomicpointer.New[State]()

	m.Logs = logs.New()

	return nil
}

func (m *Machine) GetActiveSSH() ssh.SSHClient {
	var sshClient ssh.SSHClient

	state := m.State.Load()
	switch state.ActiveSSH {
	case SSHTypeRegular:
		sshClient = m.SSH
	case SSHTypeBootstrap:
		sshClient = m.Bootstrap.SSH
	case SSHTypeKexec:
		sshClient = m.SSH
		if m.Bootstrap.SSH.IsInitialized() {
			sshClient = m.Bootstrap.SSH
		}

		sshClient.Port = m.Bootstrap.Kexec.SSHPort
		sshClient.StrictKeyChecking = false
		sshClient.DisableAutoAddHostKey = true
	}

	if !sshClient.IsInitialized() {
		panic("internal error: set active sshClient to nil")
	}

	return sshClient
}

func (m *Machine) MaybeSudo() []string {
	mi := m.MetaInspect.Load()
	if mi != nil && mi.IsRoot {
		return []string{}
	}

	return []string{m.SudoProgram.String()}
}

func (m *Machine) MaybeBootstrappingPath(restOfPath string) string {
	mi := m.MetaInspect.Load()
	if mi != nil && mi.Bootstrapped {
		return restOfPath
	}

	return "/mnt" + restOfPath
}
