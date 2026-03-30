package attributes

import (
	"os"
	"strconv"

	"dario.cat/mergo"
	config_flags "github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
	"github.com/pkg/errors"
)

const defaultFilePermissions os.FileMode = 0700

// Flake, Configuration, and Machine Attributes

type Attributes struct {
	SSH     *ssh.SSHClient              `yaml:"ssh,omitempty"`
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
		return defaultFilePermissions
	}

	return *p.PermissionsRaw
}

type Bootstrap struct {
	SSH                           *ssh.SSHClient              `yaml:"ssh,omitempty" desc:"Bootstrap SSH configuration (used during initial provisioning)"`                                                                                                               //nolint:lll
	DiskEncryptionKeys            []*PlainFileOrDirToTransfer `yaml:"disk_encryption_keys,omitempty" desc:"Keys are transferred to root dir on remote, which is the installer. If you want them to be transferred to disk of the final system, prefix path with '/mnt'"` //nolint:lll
	PostBootstrapHooks            []PostBootstrapHookCommand  `yaml:"post_bootstrap_hooks,omitempty" desc:"Commands to run after disko partitioning"`
	PostBootstrapInstallHooks     []PostBootstrapHookCommand  `yaml:"post_bootstrap_install_hooks,omitempty" desc:"Commands to run after nixos-install (before reboot)"`                                                                                                             //nolint:lll
	PostBootstrapProvisionedHooks []PostBootstrapHookCommand  `yaml:"post_bootstrap_provisioned_hooks,omitempty" desc:"Commands to run after reboot (uses regular SSH)"`                                                                                                             //nolint:lll
	Kexec                         *KexecConfig                `yaml:"kexec,omitempty" desc:"Kexec configuration for bootstrapping non-NixOS machines or reinstalling a live NixOS installation"`                                                                                     //nolint:lll
	DisableAutomaticReboot        bool                        `yaml:"disable_automatic_reboot" desc:"Disable automatic reboot after nixos-install (useful for manual inspection or custom reboot handling)"`                                                                         //nolint:lll
	ForceBootstrap                bool                        `yaml:"force_bootstrap" desc:"Force bootstrap even if machine is already NixOS (requires allow_destructive_actions)" validate:"required_if=ForceBootstrapKexec true"`                                                  //nolint:lll
	ForceBootstrapKexec           bool                        `yaml:"force_bootstrap_kexec" desc:"Force kexec method even if already in NixOS installer (requires force_bootstrap and allow_destructive_actions)"`                                                                   //nolint:lll
	AllowDestructiveActions       bool                        `yaml:"allow_destructive_actions" desc:"Allow destructive bootstrap actions (required for force_bootstrap and force_bootstrap_kexec)" validate:"required_if=ForceBootstrap true,required_if=ForceBootstrapKexec true"` //nolint:lll
}

type KexecConfig struct {
	URL        string `yaml:"url" desc:"URL or path to kexec tarball for bootstrapping non-NixOS machines" validate:"omitempty,url|filepath"`
	ExtraFlags string `yaml:"extra_flags" desc:"Extra flags to pass to kexec (e.g. '--no-sync')"`
	SSHPort    uint16 `yaml:"ssh_port" desc:"SSH port for kexec installer (default: 22)"`
}

type PostBootstrapHookCommand string

const PostBootstrapHookWaitForOnline PostBootstrapHookCommand = "waitForOnline"
const PostBootstrapHookWaitForOffline PostBootstrapHookCommand = "waitForOffline"

func (a *Attributes) Init(name string, parentAttr *Attributes, isMachine bool) error {
	err := a.PassAttributesInto(name, parentAttr)
	if err != nil {
		return err
	}

	if !isMachine {
		return nil
	}

	sshConfig, err := ssh.GetCachedSSHConfig()
	if err != nil {
		return errors.Wrapf(err, "%s", strconv.Quote(a.Xpath.String()))
	}

	// Initialize regular SSH with defaults: strict key checking enabled, auto-add disabled
	err = a.initRegularSSH(sshConfig, name)
	if err != nil {
		return err
	}

	// Initialize bootstrap SSH with defaults: strict key checking disabled, auto-add enabled
	err = a.initBootstrapSSH(sshConfig, name)
	if err != nil {
		return err
	}

	return nil
}

// initRegularSSH initializes the regular SSH configuration for this machine.
// Sets defaults when both StrictKeyChecking and DisableAutoAddHostKey are unset.
func (a *Attributes) initRegularSSH(sshConfig *ssh.SSHConfig, name string) error {
	if a.SSH == nil {
		a.SSH = &ssh.SSHClient{}
	}

	// Set defaults for regular SSH: strict key checking enabled, auto-add disabled
	// This runs when both fields are unset (false from YAML parsing)
	if !a.SSH.StrictKeyChecking && !a.SSH.DisableAutoAddHostKey {
		a.SSH.StrictKeyChecking = true
	}

	err := a.SSH.Init(sshConfig, name, a.Flags.OverrideLocalMachine)
	if err != nil {
		return errors.Wrapf(errors.Wrap(err, "ssh"), "%s", strconv.Quote(a.Xpath.String()))
	}

	return nil
}

// initBootstrapSSH initializes the bootstrap SSH configuration if present.
// Sets defaults when both StrictKeyChecking and DisableAutoAddHostKey are unset.
func (a *Attributes) initBootstrapSSH(sshConfig *ssh.SSHConfig, name string) error {
	if a.Bootstrap.SSH == nil {
		return nil
	}

	// Set defaults for bootstrap SSH: strict key checking disabled, auto-add enabled
	// This runs when both fields are unset (false from YAML parsing)
	if !a.Bootstrap.SSH.StrictKeyChecking && !a.Bootstrap.SSH.DisableAutoAddHostKey {
		a.Bootstrap.SSH.DisableAutoAddHostKey = true
	}

	err := a.Bootstrap.SSH.Init(sshConfig, name, a.Flags.OverrideLocalMachine)
	if err != nil {
		return errors.Wrapf(errors.Wrap(err, "bootstrap ssh"), "%s", strconv.Quote(a.Xpath.String()))
	}

	return nil
}

// PassAttributesInto has to be run before rest of the Init.
func (a *Attributes) PassAttributesInto(name string, parentAttr *Attributes) error {
	err := mergo.Merge(a, parentAttr, mergo.WithAppendSlice)
	if err != nil {
		return errors.Wrap(err, "failed to merge attributes")
	}

	// Deep copy SSH pointers to avoid sharing between machines
	if a.SSH != nil {
		sshCopy := *a.SSH
		a.SSH = &sshCopy
	}

	if a.Bootstrap.SSH != nil {
		sshCopy := *a.Bootstrap.SSH
		a.Bootstrap.SSH = &sshCopy
	}

	// Custom set/merge
	a.Name = name
	a.Tags = append(a.Tags, name)
	a.Xpath = parentAttr.Xpath.NewXpathWithAppend(name)

	return nil
}
