package machine

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
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
	Phase     phases.Phase     `json:"phase"`
	Duration  time.Duration    `json:"duration"`
	Error     string           `json:"error,omitempty"`
	ActiveSSH SSHType          `json:"active_ssh"`
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
		activeSSH := m.SSH
		if m.Bootstrap.SSH != nil {
			activeSSH = m.Bootstrap.SSH
		}

		port := ssh.DefaultSSHPort // default port 22
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

	return nil
}

func (m *Machine) Init(name string, parentAttributes *attributes.Attributes) error {
	err := m.Attributes.Init(name, parentAttributes, true)
	if err != nil {
		return errors.Wrap(err, "failed to initialize machine")
	}

	m.MetaInspect.Set(&MetaInspect{})
	m.State.ActiveSSH = SSHTypeRegular

	if m.Flags.ActivationMode != flags.ActivationModeSwitch {
		m.ActivationMode = m.Flags.ActivationMode
	}

	m.Logs.PhaseLogs, err = phase.NewPhaseLogs(m.Xpath)
	if err != nil {
		return errors.Wrap(err, "failed to initialize machine logs")
	}

	return nil
}

func (m *Machine) CalculateDurationAndError(workflowPhases []phases.Phase) logs.DurationAndError {
	m.Logs.CalculateDurationAndError(workflowPhases)
	m.State = m.ComputeMachineState(workflowPhases)

	return m.Logs.DurationAndErrorCache
}

func (m *Machine) ComputeMachineState(orderedPhases []phases.Phase) *State {
	machineState := &State{}

	for _, p := range orderedPhases {
		pl := m.Logs.PhaseLogs.Get(p)
		if pl == nil {
			continue
		}

		tas := pl.TimeAndState()
		machineState.Phase = p

		if !tas.IsFinished() {
			machineState.Status = stats.Running
			machineState.Duration, _ = tas.DurationOrElapsedTime()

			return machineState
		}

		duration, _ := tas.DurationOrElapsedTime()
		machineState.Duration += duration

		endErr := tas.GetEndError()
		if endErr != nil {
			machineState.Status = stats.Failed
			machineState.Error = endErr.Error()

			return machineState
		}
	}

	if machineState.Phase != "" {
		machineState.Status = stats.Done
	}

	return machineState
}

func (m *Machine) GetCurrentTargetLog() *phase.PhaseLog {
	for _, phaseLogPair := range m.Logs.PhaseLogs.All() {
		phaseLog := phaseLogPair.Value

		err := phaseLog.TimeAndState().GetEndError()
		if err != nil {
			return phaseLog
		}
	}

	return m.Logs.PhaseLogs.Last()
}

var (
	emptySudo = []string{}
	sudoCmd   = []string{"sudo"}
)

func (m *Machine) MaybeSudo() []string {
	mi := m.MetaInspect.Get()
	if mi != nil && mi.IsRoot {
		return emptySudo
	}

	if m.OverrideSudoProgram == "" {
		return sudoCmd
	}

	return []string{m.OverrideSudoProgram}
}

func (m *Machine) MaybeBootstrappingPath(restOfPath string) string {
	mi := m.MetaInspect.Get()
	if mi != nil && mi.Bootstrapped {
		return restOfPath
	}

	return "/mnt" + restOfPath
}
