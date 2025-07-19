package config

import (
	"net/url"
	"strings"
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
}

// Configuration

type Configuration struct {
	FlakeOutput       string `yaml:"flakeOutput"` // Override if not standard style
	DefaultAttributes `yaml:",inline"`
	Machines          *orderedmap.OrderedMap[url.URL, *Machine] `yaml:"machines"` // Key here is the ssh URL: alias, user@host or user@host:port
	// Meta
	Logs   map[workflow_definition.WorkflowPhase]*Log
	Phases ConfigurationPhases
}

type CommandLog struct {
	Command     string
	Stdout      strings.Builder
	Stderr      strings.Builder
	StdCombined strings.Builder
	*TimeAndState
}

type SshClient struct {
	Url        *url.URL `yaml:"-"`
	PrivateKey string   `yaml:"privateKey"`
	PublicKey  string   `yaml:"publicKey"`
}

type ConfigurationPhases struct {
	PhaseBuild *PhaseBuild
}

type PhaseBuild struct {
	BuildOutputPath string
}

// Machine

type Machine struct {
	DefaultAttributes `yaml:",inline"`
	// Meta
	Logs   map[workflow_definition.WorkflowPhase]*Log
	Phases *MachinePhases
}

type Log struct {
	Commands     []*CommandLog
	TimeAndState *TimeAndState
}

func (log *Log) LastCommand() *CommandLog {
	if len(log.Commands) == 0 {
		return &CommandLog{TimeAndState: &TimeAndState{}}
	}

	return log.Commands[len(log.Commands)-1]
}

type MachinePhases struct {
	Status *PhaseStatus
	//Transfer
	//Secrets
	//Activation
}

type PhaseStatus struct {
	Reachable         bool
	SSHConnectable    bool
	Bootstrapped      bool
	CurrentGeneration string
	LastDeployTime    string
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
