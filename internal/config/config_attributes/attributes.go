package config_attributes

import (
	"slices"
	"strconv"

	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/pkg/errors"
)

// Flake, Configuration and Machine Attributes

type Attributes struct {
	SSH                 *ssh.SshClient  `yaml:"ssh,omitempty"`
	Tags                []string        `yaml:"tags"`
	Secrets             []*SecretConfig `yaml:"secrets,omitempty"`
	Disabled            bool            `yaml:"disabled"`
	OverrideSudoProgram string          `yaml:"override_sudo_program"`
	HardwareConfigPath  string          `yaml:"hardware_config_path"`

	PostBootstrapHook string `yaml:"post_bootstrap_hook"`

	// Internal
	Name    string              `yaml:"-"`
	Xpath   Xpath               `yaml:"-"`
	Message string              `yaml:"-"`
	Flags   *config_flags.Flags `yaml:"-"`
}

func (a *Attributes) Init(name string, attr *Attributes, flags *config_flags.Flags) (err error) {
	a.Name = name
	a.Flags = flags

	a.Tags = append(a.Tags, name)

	a.PassAttributesInto(attr)

	defer func() {
		err = errors.Wrapf(err, "%s", strconv.Quote(a.Xpath.String()))
	}()

	sshConfig, err := ssh.GetCachedSshConfig()
	if err != nil {
		return
	}

	err = a.SSH.Init(sshConfig, name, flags.OverrideLocalMachine)
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
func (a *Attributes) PassAttributesInto(parentAttr *Attributes) {
	if a.SSH == nil {
		a.SSH = parentAttr.SSH
	}

	a.Tags = append(a.Tags, parentAttr.Tags...)
	a.Tags = slices.Compact(a.Tags)

	a.Secrets = append(a.Secrets, parentAttr.Secrets...)

	if parentAttr.Disabled {
		a.Disabled = true
	}

	if a.OverrideSudoProgram == "" {
		a.OverrideSudoProgram = parentAttr.OverrideSudoProgram
	}

	if a.HardwareConfigPath == "" {
		a.HardwareConfigPath = parentAttr.HardwareConfigPath
	}

	a.Xpath = parentAttr.Xpath.NewXpathWithAppend(a.Name)
}
