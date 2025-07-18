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
	Global Global                                 `koanf:"global"`
	Flakes *orderedmap.OrderedMap[string, *Flake] `koanf:"flakes"`
}

type Global struct {
	Filters           Filters                             `koanf:"filters"`
	RequireAllSuccess bool                                `koanf:"requireAllSuccess"`
	AutoBootstrap     bool                                `koanf:"autoBootstrap"`
	LocalMachine      string                              `koanf:"localMachine"`
	DryRun            bool                                `koanf:"dryRun"`
	Ssh               *SshClient                          `koanf:"ssh"`
	Timeout           time.Duration                       `koanf:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int                                 `koanf:"concurrency"`
	SkipPhases        []workflow_definition.WorkflowPhase `koanf:"skipPhases"`
	Verbose           bool                                `koanf:"verbose"`
	Debug             bool                                `koanf:"debug"`
	//Json              bool                                `koanf:"json"` // Maybe later
}

type Filters struct {
	Flakes         []string `koanf:"flakes"`
	Configurations []string `koanf:"configurations"`
	Machines       []string `koanf:"machines"`
	Tags           []string `koanf:"tags"`
}

type Flake struct {
	Url               string `koanf:"url"` // Flake path or url
	DefaultAttributes `koanf:",squash"`
	Configurations    *orderedmap.OrderedMap[string, *Configuration] `koanf:"configurations"`
}

// Configuration

type Configuration struct {
	FlakeOutput       string `koanf:"flakeOutput"` // Override if not standard style
	DefaultAttributes `koanf:",squash"`
	Machines          *orderedmap.OrderedMap[url.URL, *Machine] `koanf:"machines"` // Key here is the ssh URL: alias, user@host or user@host:port
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
	Url        *url.URL `koanf:"-"`
	PrivateKey string   `koanf:"privateKey"`
	PublicKey  string   `koanf:"publicKey"`
}

type ConfigurationPhases struct {
	PhaseBuild *PhaseBuild
}

type PhaseBuild struct {
	BuildOutputPath string
}

// Machine

type Machine struct {
	DefaultAttributes `koanf:",squash"`
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
		return &CommandLog{}
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
	Ssh      *SshClient      `koanf:"ssh,omitempty"`
	Tags     []string        `koanf:"tags"`
	Secrets  []*SecretConfig `koanf:"secrets,omitempty"`
	Disabled bool            `koanf:"disabled"` // This attribute does not play any role after filtering
}

type SecretConfig struct {
	LocalPath  string `koanf:"localPath"`
	RemotePath string `koanf:"remotePath"`
	Mode       string `koanf:"mode"`
}
