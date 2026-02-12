package config_attributes

import (
	"slices"
	"strconv"

	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/pkg/errors"
)

// Flake, Configuration, and Machine Attributes

type Attributes struct {
	SSH     *ssh.SshClient `yaml:"ssh,omitempty"`
	Tags    []string       `yaml:"tags"`
	Secrets []*Secret      `yaml:"secrets,omitempty"`

	Disabled            bool   `yaml:"disabled"`
	OverrideSudoProgram string `yaml:"override_sudo_program"`
	HardwareConfigPath  string `yaml:"hardware_config_path"`

	Bootstrap Bootstrap `yaml:"bootstrap"`

	// Internal
	Name    string              `yaml:"-"`
	Xpath   Xpath               `yaml:"-"`
	Message string              `yaml:"-"`
	Flags   *config_flags.Flags `yaml:"-"`
}

type Bootstrap struct {
	DiskEncryptionKeys []*DiskEncryptionKey `yaml:"disk_encryption_keys,omitempty"`
	PostBootstrapHook  string               `yaml:"post_bootstrap_hook"`
}

func (a *Attributes) Init(name string, attr *Attributes, flags *config_flags.Flags) error {
	a.Name, a.Flags = name, flags
	a.Tags = append(a.Tags, name)
	a.PassAttributesInto(attr)

	sshConfig, err := ssh.GetCachedSshConfig()
	if err != nil {
		return errors.Wrapf(err, "%s", strconv.Quote(a.Xpath.String()))
	}

	err = a.SSH.Init(sshConfig, name, flags.OverrideLocalMachine)
	if err != nil {
		return errors.Wrapf(errors.Wrap(err, "ssh"), "%s", strconv.Quote(a.Xpath.String()))
	}

	for _, diskEncryptionKey := range a.Bootstrap.DiskEncryptionKeys {
		err = diskEncryptionKey.Validate()
		if err != nil {
			return errors.Wrapf(err, "%s", strconv.Quote(a.Xpath.String()))
		}
	}

	for _, secret := range a.Secrets {
		err = secret.Validate()
		if err != nil {
			return errors.Wrapf(err, "%s", strconv.Quote(a.Xpath.String()))
		}
	}

	return nil
}

// PassAttributesInto has to be run before rest of the Init
func (a *Attributes) PassAttributesInto(parentAttr *Attributes) {
	if a.SSH == nil {
		a.SSH = parentAttr.SSH
	}
	a.Tags = slices.Compact(append(a.Tags, parentAttr.Tags...))
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
	if len(a.Bootstrap.DiskEncryptionKeys) == 0 {
		a.Bootstrap.DiskEncryptionKeys = parentAttr.Bootstrap.DiskEncryptionKeys
	}
	a.Xpath = parentAttr.Xpath.NewXpathWithAppend(a.Name)
}
