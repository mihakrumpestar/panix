package config_attributes

import (
	"os"
	"strconv"

	"dario.cat/mergo"
	"github.com/mihakrumpestar/panix/internal/config/config_flags"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/pkg/errors"
)

// Flake, Configuration, and Machine Attributes

type Attributes struct {
	SSH     *ssh.SshClient              `yaml:"ssh,omitempty"`
	Tags    []string                    `yaml:"tags"`
	Secrets []*PlainFileOrDirToTransfer `yaml:"secrets,omitempty"`

	Disabled            bool   `yaml:"disabled"`
	OverrideSudoProgram string `yaml:"override_sudo_program"`
	HardwareConfigPath  string `yaml:"hardware_config_path"`

	Bootstrap Bootstrap `yaml:"bootstrap"`

	Name    string              `yaml:"-" validate:"-"`
	Xpath   Xpath               `yaml:"-" validate:"-"`
	Message string              `yaml:"-" validate:"-"`
	Flags   *config_flags.Flags `yaml:"-" validate:"-"`
}

type PlainFileOrDirToTransfer struct {
	LocalPath      string       `yaml:"local_path,required" desc:"Path to a local file or dir" validate:"required,filepath"`
	RemotePath     string       `yaml:"remote_path,required" desc:"Absolute path on remote machine" validate:"required,abspath"`
	UID            *uint        `yaml:"uid,omitempty" desc:"Optional User ID for remote" validate:"required_with=GID"`
	GID            *uint        `yaml:"gid,omitempty" desc:"Optional Group ID for remote" validate:"required_with=UID"`
	PermissionsRaw *os.FileMode `yaml:"permissions,omitempty" desc:"Optional file permissions (default: 0700)"`
}

func (p *PlainFileOrDirToTransfer) GetPermissions() os.FileMode {
	if p.PermissionsRaw == nil {
		return 0700
	}
	return *p.PermissionsRaw
}

type Bootstrap struct {
	DiskEncryptionKeys []*PlainFileOrDirToTransfer `yaml:"disk_encryption_keys,omitempty"`
	PostBootstrapHook  string                      `yaml:"post_bootstrap_hook"`
	KexecURL           string                      `yaml:"kexec_url" desc:"URL or path to kexec tarball for bootstrapping non-NixOS machines" validate:"omitempty,url|filepath"`
	KexecExtraFlags    string                      `yaml:"kexec_extra_flags" desc:"Extra flags to pass to kexec (e.g. '--no-sync')"`
}

func (a *Attributes) Init(name string, parentAttr *Attributes, isMachine bool) error {
	err := a.PassAttributesInto(name, parentAttr)
	if err != nil {
		return err
	}

	if !isMachine {
		return nil
	}

	sshConfig, err := ssh.GetCachedSshConfig()
	if err != nil {
		return errors.Wrapf(err, "%s", strconv.Quote(a.Xpath.String()))
	}

	if a.SSH == nil {
		a.SSH = &ssh.SshClient{}
	}

	err = a.SSH.Init(sshConfig, name, a.Flags.OverrideLocalMachine)
	if err != nil {
		return errors.Wrapf(errors.Wrap(err, "ssh"), "%s", strconv.Quote(a.Xpath.String()))
	}

	return nil
}

// PassAttributesInto has to be run before rest of the Init
func (a *Attributes) PassAttributesInto(name string, parentAttr *Attributes) error {
	err := mergo.Merge(a, parentAttr, mergo.WithAppendSlice)
	if err != nil {
		return err
	}

	// Custom set/merge
	a.Name = name
	a.Tags = append(a.Tags, name)
	a.Xpath = parentAttr.Xpath.NewXpathWithAppend(name)

	return nil
}
