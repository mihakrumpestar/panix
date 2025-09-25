package config

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

const (
	ContextConfigKey = "config"
)

type Config struct {
	Global Global `yaml:"global"`
	Root   *Root  `yaml:"root"`
}

type Global struct {
	Filters           Filters                             `yaml:"filters"`
	RequireAllSuccess bool                                `yaml:"requireAllSuccess"`
	AutoBootstrap     bool                                `yaml:"autoBootstrap"`
	LocalMachine      string                              `yaml:"localMachine"`
	DryRun            bool                                `yaml:"dryRun"`
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

type Root struct {
	Flakes      SortableFoCoM[string, *Flake] `yaml:"flakes"`
	*Attributes `yaml:",inline"`
}

func (r *Root) Init(name string) {
	r.Attributes = NewAttributes("")
}

func (r *Root) Children(skipDisabled bool) []FoCoM {
	result := make([]FoCoM, 0)
	for _, config := range r.Flakes.SortedMap(false, skipDisabled) {
		result = append(result, config)
	}
	return result
}

type Flake struct {
	Url            string `yaml:"url"` // Flake path or url
	*Attributes    `yaml:",inline"`
	Configurations SortableFoCoM[string, *Configuration] `yaml:"configurations"`
	Hooks          Hooks                                 `yaml:",buildHooks"` // They only run for builds
}

func (f *Flake) Init(name string) {
	f.Attributes = NewAttributes(name)
}

func (f *Flake) Children(skipDisabled bool) []FoCoM {
	result := make([]FoCoM, 0)
	for _, config := range f.Configurations.SortedMap(false, skipDisabled) {
		result = append(result, config)
	}
	return result
}

type Hooks struct {
	Pre  string `yaml:",pre"`
	Post string `yaml:",post"`
}

// Configuration

type Configuration struct {
	FlakeOutput string `yaml:"flakeOutput"` // Option to override if non-standard style
	*Attributes `yaml:",inline"`
	Machines    SortableFoCoM[string, *Machine] `yaml:"machines"`
	// Meta
	Phases *ConfigurationPhases
}

func (c *Configuration) Init(name string) {
	c.Attributes = NewAttributes(name)

	c.Phases = &ConfigurationPhases{
		Build: &PhaseBuild{},
	}
}

func (r *Configuration) Children(skipDisabled bool) []FoCoM {
	result := make([]FoCoM, 0)
	for _, config := range r.Machines.SortedMap(false, skipDisabled) {
		result = append(result, config)
	}
	return result
}

type ConfigurationPhases struct {
	Build *PhaseBuild
}

type PhaseBuild struct {
	BuildOutputPath string
}

// Machine

type Machine struct {
	*Attributes `yaml:",inline"`
	// Meta
	Phases *MachinePhases
}

func (m *Machine) Init(name string) {
	m.Attributes = NewAttributes(name)

	if m.Attributes.Ssh == nil { // Only machine has them always initialized (root, flake, configurations do not)
		m.Attributes.Ssh = &SshClient{}
	}

	m.Phases = &MachinePhases{
		Status: &PhaseStatus{},
	}
}

func (m *Machine) Children(skipDisabled bool) []FoCoM {
	panic("This should not be called as machine has not children")
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

	// Metadata attributes (not provided by the user, by by runtime)
	Name    string
	Message string
	Logs    *Logs
}

func NewAttributes(name string) *Attributes {
	return &Attributes{
		Name: name,
		Logs: NewLogs(),
	}
}

func (a *Attributes) Disable(msg string) {
	a.Disabled = true
	a.Message = msg
}

func (a *Attributes) IsDisabled() bool {
	return a.Disabled
}

func (a *Attributes) Msg(msg string) {
	a.Message = msg
}

type SecretConfig struct {
	LocalPath  string `yaml:"localPath"`
	RemotePath string `yaml:"remotePath"`
}
