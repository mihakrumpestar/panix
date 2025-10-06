package config_attributes

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/pkg/errors"
)

// Flake, Configuration and Machine Attributes

type Attributes struct {
	Ssh                *ssh.SshClient  `yaml:"ssh,omitempty"`
	Tags               []string        `yaml:"tags"`
	Secrets            []*SecretConfig `yaml:"secrets,omitempty"`
	Disabled           bool            `yaml:"disabled"`
	SudoProgram        *string         `yaml:"sudo_program,omitempty"` // Default is "sudo", if specified (but empty string) it will disable privilidge escalation altogether
	HardwareConfigPath string          `yaml:"hardware_config_path"`

	// Internal
	Name    string
	Xpath   string
	Message string
	Logs    *logs.PhaseLogs
	Flags   *config_flags.Flags
}

func (a *Attributes) Init(name string, passAttr *Attributes, flags *config_flags.Flags) (err error) {
	defer func() {
		err = errors.Wrapf(err, "%s", strconv.Quote(a.Xpath))
	}()

	a.Name = name
	a.Logs = logs.NewPhaseLogs(flags)
	a.Flags = flags

	a.PassAttributesInto(passAttr)

	sshConfig, err := ssh.GetCachedSshConfig()
	if err != nil {
		return
	}

	err = a.Ssh.Init(sshConfig, name, flags.OverrideLocalMachine)
	if err != nil {
		return errors.Wrap(err, "ssh")
	}

	for _, secret := range a.Secrets {
		err = secret.Validate()
		if err != nil {
			return
		}
	}

	return nil
}

// PassAttributesInto has to be run before rest of the Init
func (a *Attributes) PassAttributesInto(attr *Attributes) {
	if a.Ssh == nil {
		a.Ssh = attr.Ssh
	}

	a.Tags = append(a.Tags, attr.Tags...)
	a.Tags = slices.Compact(a.Tags)

	a.Secrets = append(a.Secrets, attr.Secrets...)

	if attr.Disabled {
		a.Disabled = true
	}

	if a.SudoProgram == nil {
		a.SudoProgram = attr.SudoProgram
	}

	if a.HardwareConfigPath == "" {
		a.HardwareConfigPath = attr.HardwareConfigPath
	}

	a.Xpath = a.Name
	if attr.Xpath != "" {
		a.Xpath = attr.Xpath + "/" + a.Name
	}
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
		return errors.New("remote secrets path is empty")
	}

	if !strings.HasPrefix(sc.Remote.Path, "/") {
		return fmt.Errorf("remote secrets path must be absolute for %s", strconv.Quote(sc.Remote.Path))
	}

	return nil
}
