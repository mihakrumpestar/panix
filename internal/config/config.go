package config

import (
	"iter"
	"net/url"
	"time"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

const (
	ContextConfigKey = "config"
)

type Config struct {
	Global Global                                 `yaml:"global"`
	Flakes *orderedmap.OrderedMap[string, *Flake] `yaml:"flakes"`
}

type Global struct {
	Filters           Filters                             `yaml:"filters"`
	RequireAllSuccess bool                                `yaml:"requireAllSuccess"`
	AutoBootstrap     bool                                `yaml:"autoBootstrap"`
	LocalMachine      string                              `yaml:"localMachine"`
	DryRun            bool                                `yaml:"dryRun"`
	Ssh               *SshClient                          `yaml:"ssh"`
	Timeout           time.Duration                       `yaml:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int                                 `yaml:"concurrency"`
	SkipPhases        []workflow_definition.WorkflowPhase `yaml:"skipPhases"`
	Verbose           bool                                `yaml:"verbose"`
	Debug             bool                                `yaml:"debug"`
	//Json              bool                                `yaml:"json"` // Maybe later
}

type Filters struct {
	Flakes         []string `yaml:"flakes"`
	Configurations []string `yaml:"configurations"`
	Machines       []string `yaml:"machines"`
	Tags           []string `yaml:"tags"`
}

type Flake struct {
	Url               string `yaml:"url"` // Flake path or url
	DefaultAttributes `yaml:",inline"`
	Configurations    *orderedmap.OrderedMap[string, *Configuration] `yaml:"configurations"`
	BuildHooks        BuildHooks                                     `yaml:",buildHooks"` // They only run for builds
	// Meta
	Logs *Logs
}

type BuildHooks struct {
	Pre  string `yaml:",pre"`
	Post string `yaml:",post"`
}

// Configuration

type Configuration struct {
	FlakeOutput       string `yaml:"flakeOutput"` // Override if not standard style
	DefaultAttributes `yaml:",inline"`
	Machines          *orderedmap.OrderedMap[url.URL, *Machine] `yaml:"machines"` // Key here is the ssh URL: alias, user@host or user@host:port
	// Meta
	Logs   *Logs
	Phases *ConfigurationPhases
}

type SshClient struct {
	Url        *url.URL `yaml:"-"`
	PrivateKey string   `yaml:"privateKey"`
	PublicKey  string   `yaml:"publicKey"`
}

type ConfigurationPhases struct {
	Build *PhaseBuild
}

type PhaseBuild struct {
	BuildOutputPath string
}

// Machine

type Machine struct {
	DefaultAttributes `yaml:",inline"`
	// Meta
	Logs   *Logs
	Phases *MachinePhases
}

type Logs struct {
	logs *orderedmap.OrderedMap[workflow_definition.WorkflowPhase, *Log]
}

func NewLogs() *Logs {
	om := orderedmap.NewOrderedMap[workflow_definition.WorkflowPhase, *Log]()
	return &Logs{om}
}

func (l *Logs) SafeGet(wp workflow_definition.WorkflowPhase) *Log {
	log, ok := l.logs.Get(wp)

	if !ok {
		log = &Log{
			Commands:     make([]*CommandLog, 0),
			TimeAndState: &TimeAndState{},
		}

		l.logs.Set(wp, log)
	}

	return log
}

func (l *Logs) All() iter.Seq2[workflow_definition.WorkflowPhase, *Log] {
	return l.logs.AllFromFront()
}

func (l *Logs) Len() int {
	return l.logs.Len()
}

type Log struct {
	Commands     []*CommandLog
	TimeAndState *TimeAndState
}

func (log *Log) LastCommand() *CommandLog {
	// Safety
	if len(log.Commands) == 0 {
		return &CommandLog{TimeAndState: TimeAndState{}}
	}

	return log.Commands[len(log.Commands)-1]
}

func (log *Log) NewCommand() *CommandLog {
	cmd := &CommandLog{TimeAndState: TimeAndState{}}

	log.Commands = append(log.Commands, cmd)

	return cmd
}

func (log *Log) AddMessageOnly(msg ...string) {
	comLog := &CommandLog{
		Command: "-> ",
	}

	for _, msgInstance := range msg {
		comLog.Command += msgInstance
	}

	comLog.TimeAndState.StartTimer()
	comLog.TimeAndState.EndTimer()

	if log.Commands == nil {
		log.Commands = make([]*CommandLog, 0)
	}

	log.Commands = append(log.Commands, comLog)
}

type MachinePhases struct {
	Status *PhaseStatus
	//Transfer
	//Secrets
	//Activation
}

type PhaseStatus struct {
	Reachable      bool
	SSHConnectable bool
	Bootstrapped   bool
	Generation     string
	Date           string
	Nixos          string
	Kernel         string
}

// Configuration and Machine

type DefaultAttributes struct {
	Ssh      *SshClient      `yaml:"ssh,omitempty"`
	Tags     []string        `yaml:"tags"`
	Secrets  []*SecretConfig `yaml:"secrets,omitempty"`
	Disabled bool            `yaml:"disabled"` // This attribute does not play any role after filtering
}

type SecretConfig struct {
	LocalPath  string `yaml:"localPath"`
	RemotePath string `yaml:"remotePath"`
	Mode       string `yaml:"mode"`
}
