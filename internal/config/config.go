package config

import (
	"errors"
	"time"

	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const (
	ContextConfigKey = "config"
)

type Config struct {
	Global Global `yaml:"global"`
	Root   *Root  `yaml:"root"`
}

type Global struct {
	Filters           Filters        `yaml:"filters"`
	RequireAllSuccess bool           `yaml:"requireAllSuccess"`
	AutoBootstrap     bool           `yaml:"autoBootstrap"`
	LocalMachine      string         `yaml:"localMachine"`
	DryRun            bool           `yaml:"dryRun"`
	Timeout           time.Duration  `yaml:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int            `yaml:"concurrency"`
	SkipPhases        []phases.Phase `yaml:"skipPhases"`
	Verbose           bool           `yaml:"verbose"`
	Debug             bool           `yaml:"debug"`
	//Json              bool                                `yaml:"json"` // Maybe later
}

type Filters struct {
	Flakes         []string `yaml:"flakes"`
	Configurations []string `yaml:"configurations"`
	Machines       []string `yaml:"machines"`
	Tags           []string `yaml:"tags"`
}

// Root

type Root struct {
	Flakes     SortedMap[string, *Flake] `yaml:"flakes"`
	Attributes `yaml:",squash"`
}

func (r *Root) Init(name string) error {
	err := r.Attributes.InitAttributes("")
	if err != nil {
		return err
	}

	return nil
}

// Flake

type Flake struct {
	Url            string `yaml:"url"` // Flake path or url
	Attributes     `yaml:",squash"`
	Configurations SortedMap[string, *Configuration] `yaml:"configurations"`
	BuildHooks     BuildHooks                        `yaml:"buildHooks"` // They only run for builds
}

func (f *Flake) Init(name string) error {
	err := f.Attributes.InitAttributes(name)
	if err != nil {
		return err
	}

	return nil
}

type BuildHooks struct {
	Pre  string `yaml:",pre"`
	Post string `yaml:",post"`
}

// Configuration

type Configuration struct {
	FlakeOutput string `yaml:"flakeOutput"` // Option to override if non-standard style
	Attributes  `yaml:",squash"`
	Machines    SortedMap[string, *Machine] `yaml:"machines"`
	// Internal
	MetaBuild MetaBuild
}

type MetaBuild struct {
	OutputPath string
}

func (c *Configuration) Init(name string) error {
	err := c.Attributes.InitAttributes(name)
	if err != nil {
		return err
	}

	return nil
}

// Machine

type Machine struct {
	Attributes `yaml:",squash"`
	// Internal
	MetaStatus *MetaStatus
}

type MetaStatus struct {
	Reachable      bool
	SSHConnectable bool
	Bootstrapped   bool
	Generation     string
	Date           string
	Nixos          string
	Kernel         string
}

func (m *Machine) Init(name string) error {
	err := m.Attributes.InitAttributes(name)
	if err != nil {
		return err
	}

	// Only machine has them always initialized (root, flake, configurations do not)

	if m.Attributes.Ssh == nil {
		m.Attributes.Ssh = &SshClient{}
	}

	if m.Attributes.SudoProgram == nil {
		sudoProgram := "sudo" // Default sudo program
		m.Attributes.SudoProgram = &sudoProgram
	}

	if m.MetaStatus == nil {
		m.MetaStatus = &MetaStatus{}
	}

	return nil
}

// Flake, Configuration and Machine Attributes

type Attributes struct {
	Ssh         *SshClient      `yaml:"ssh,omitempty"`
	Tags        []string        `yaml:"tags"`
	Secrets     []*SecretConfig `yaml:"secrets,omitempty"`
	Disabled    bool            `yaml:"disabled"`
	SudoProgram *string         `yaml:"sudo_program"` // Default it is "sudo", if specified (but empty string), it will disable privilidge escalation altogether

	// Internal
	Name    string
	Message string
	Logs    *PhaseLogs
}

func (a *Attributes) InitAttributes(name string) error {
	a.Name = name
	a.Logs = NewPhaseLogs()

	for _, secret := range a.Secrets {
		err := secret.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

type SecretConfig struct {
	Local  Local  `yaml:"local"`
	Remote Remote `yaml:"remote"`
}

type Local struct {
	Path          *string `yaml:"path"`
	CommandOutput *string `yaml:"commandOutput"`
}

type Remote struct {
	Path string `yaml:"path"`
	UID  *uint  `yaml:"uid,omitempty"`
	GID  *uint  `yaml:"gid,omitempty"`
}

func (sc *SecretConfig) Validate() error {
	if sc.Local.Path == nil && sc.Local.CommandOutput == nil {
		return errors.New("both local input socrets options are empty")
	}

	if sc.Local.Path != nil && sc.Local.CommandOutput != nil {
		return errors.New("can't use both local input socrets options")
	}

	if sc.Remote.Path == "" {
		return errors.New("remote socrets path is empty")
	}

	return nil
}
