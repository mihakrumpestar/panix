package machine

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type SSHType string

const (
	SSHTypeBootstrap SSHType = "bootstrap"
	SSHTypeKexec     SSHType = "kexec"
	SSHTypeRegular   SSHType = "regular"
)

type Machine struct {
	attributes.Attributes `yaml:",inline"`

	// Internal
	MetaInspect atomicpointer.AtomicPointer[MetaInspect] `yaml:"-" json:"meta_inspect,omitempty" validate:"-"`
	Logs        *logs.Logs                               `yaml:"-" json:"logs,omitempty"`
	State       *State                                   `yaml:"-" json:"machine_state,omitempty"`
}

// MetaInspect needs to be atomic (updates with executioner)
type MetaInspect struct {
	Reachable      bool `json:"reachable"`
	SSHConnectable bool `json:"ssh_connectable"`
	IsRoot         bool `json:"is_root"`
	Bootstrapped   bool `json:"bootstrapped"`
	RequiresKexec  bool `json:"requires_kexec"`

	Architecture string       `json:"architecture,omitempty"`
	Generations  *Generations `json:"generations,omitempty"`
	Date         string       `json:"date,omitempty"`
	Nixos        string       `json:"nixos,omitempty"`
	Kernel       string       `json:"kernel,omitempty"`
}

// State does not need to be atomic (updates with UI)
type State struct {
	Status    stats.StatsState `json:"status"`
	StatusMsg string           `json:"status_msg"`
	Phase     phases.Phase     `json:"phase"`
	//Duration  time.Duration    `json:"duration"`
	Error     error   `json:"error,omitempty"`
	ActiveSSH SSHType `json:"active_ssh"`
}

type Generations struct {
	Current   uint   `json:"current"`
	Available []uint `json:"available"`
}

func (m *Machine) GetActiveSSH() *ssh.SSHClient {
	switch m.State.ActiveSSH {
	case SSHTypeRegular:
		return m.SSH
	case SSHTypeBootstrap:
		return m.Bootstrap.SSH

	case SSHTypeKexec:
		var activeSSH *ssh.SSHClient

		// Copy by value (dereference)
		*activeSSH = *m.SSH
		if m.Bootstrap.SSH != nil {
			*activeSSH = *m.Bootstrap.SSH
		}

		port := ssh.DefaultSSHPort // default port 22
		if m.Bootstrap.Kexec != nil && m.Bootstrap.Kexec.SSHPort != 0 {
			port = m.Bootstrap.Kexec.SSHPort
		}

		activeSSH.Port = port
		activeSSH.StrictKeyChecking = false
		activeSSH.DisableAutoAddHostKey = true

		return activeSSH
	}

	return nil
}

func (m *Machine) Init(name string, parentAttributes *attributes.Attributes) error {
	if m == nil {
		return fmt.Errorf("internal error: machine %s has nil value", name)
	}

	err := m.Attributes.Init(name, parentAttributes, true)
	if err != nil {
		return errors.Wrap(err, "failed to initialize machine")
	}

	m.MetaInspect.Clear()
	m.State = &State{ActiveSSH: SSHTypeRegular}

	m.Logs = logs.New(&m.Attributes)

	return nil
}

var (
	emptySudo = []string{}
	sudoCmd   = []string{"sudo"}
)

func (m *Machine) MaybeSudo() []string {
	mi := m.MetaInspect.Load()
	if mi != nil && mi.IsRoot {
		return emptySudo
	}

	if m.OverrideSudoProgram == "" {
		return sudoCmd
	}

	return []string{m.OverrideSudoProgram}
}

func (m *Machine) MaybeBootstrappingPath(restOfPath string) string {
	mi := m.MetaInspect.Load()
	if mi != nil && mi.Bootstrapped {
		return restOfPath
	}

	return "/mnt" + restOfPath
}
