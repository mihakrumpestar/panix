package config

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

const (
	ContextConfigKey = "config"
)

type Config struct {
	Global Global                      `yaml:"global"`
	Flakes SortableMap[string, *Flake] `yaml:"flakes"`
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
	Url            string `yaml:"url"` // Flake path or url
	Attributes     `yaml:",inline"`
	Configurations SortableMap[string, *Configuration] `yaml:"configurations"`
	Hooks          Hooks                               `yaml:",buildHooks"` // They only run for builds
	// Meta
	MetadataAttributes
}

type Hooks struct {
	Pre  string `yaml:",pre"`
	Post string `yaml:",post"`
}

// Configuration

type Configuration struct {
	FlakeOutput string `yaml:"flakeOutput"` // Option to override if non-standard style
	Attributes  `yaml:",inline"`
	Machines    SortableMap[string, *Machine] `yaml:"machines"` // Key here is the ssh URL: alias, user@host or user@host:port
	// Meta
	MetadataAttributes
	Phases *ConfigurationPhases
}

type ConfigurationPhases struct {
	Build *PhaseBuild
}

type PhaseBuild struct {
	BuildOutputPath string
}

// Machine

type Machine struct {
	Attributes `yaml:",inline"`
	// Meta
	MetadataAttributes
	Phases *MachinePhases
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

// Flake, Configuration and Machine Attributes

type Attributes struct {
	Ssh      *SshClient      `yaml:"ssh,omitempty"`
	Tags     []string        `yaml:"tags"`
	Secrets  []*SecretConfig `yaml:"secrets,omitempty"`
	Disabled bool            `yaml:"disabled"`
}

func (a *Attributes) IsDisabled() bool {
	return a.Disabled
}

type SecretConfig struct {
	LocalPath  string `yaml:"localPath"`
	RemotePath string `yaml:"remotePath"`
}

// Flake, Configuration and Machine MetadataAttributes

type MetadataAttributes struct {
	Name    string
	Message string
	Logs    *Logs
}

func (mlma *MetadataAttributes) Init(name string) {
	mlma.Name = name
	mlma.Logs = NewLogs()
}
