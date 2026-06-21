package attributes

import (
	"dario.cat/mergo"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
)

// Flake, Configuration, and Machine Attributes

//nolint:lll
type Attributes struct {
	SSH     ssh.SSHClient              `yaml:"ssh" json:"ssh" desc:"SSH configuration for remote access"`
	Tags    []string                   `yaml:"tags" json:"tags,omitempty" desc:"Tags for filtering (flakes, configs and machines are already registered as tags)"`
	Secrets []PlainFileOrDirToTransfer `yaml:"secrets" json:"secrets,omitempty" desc:"Files or directories to transfer to the remote machine" validate:"dive"`

	Disabled           bool            `yaml:"disabled" json:"disabled,omitempty" desc:"Disable this"`
	SudoProgram        SudoProgram     `yaml:"sudo_program" json:"sudo_program,omitempty" desc:"Override the sudo program" default:"sudo"`
	HardwareConfigPath string          `yaml:"hardware_config_path" json:"hardware_config_path,omitempty" desc:"Path to hardware config"`
	ActivationMode     ActivationModeD `yaml:"activation_mode" json:"activation_mode,omitempty" desc:"Activation mode: check, switch, boot, test, dry-activate" default:"switch" validate:"omitempty,oneof=check switch boot test dry-activate"`

	Bootstrap Bootstrap `yaml:"bootstrap" json:"bootstrap" desc:"Bootstrap configuration for initial provisioning"`

	Name  string      `yaml:"-" json:"name,omitempty"`
	Xpath xpath.Xpath `yaml:"-" json:"xpath,omitempty"`

	// PhaseXpaths maps each phase to its full xpath (entityXpath + "/" + phaseName).
	// Pre-computed once during Init — eliminates per-frame string concatenation
	// in the TUI render loop.
	PhaseXpaths map[phase.Phase]xpath.Xpath `yaml:"-" json:"-"`
}

type PlainFileOrDirToTransfer struct {
	LocalPath   string   `yaml:"local_path,required" json:"local_path" desc:"Path to a local file or dir" validate:"required,filepath"`
	RemotePath  string   `yaml:"remote_path,required" json:"remote_path" desc:"Absolute path on remote machine" validate:"required,abspath"`
	UID         *uint    `yaml:"uid,omitempty" json:"uid,omitempty" desc:"Optional User ID for remote" validate:"required_with=GID"`
	GID         *uint    `yaml:"gid,omitempty" json:"gid,omitempty" desc:"Optional Group ID for remote" validate:"required_with=UID"`
	Permissions FileMode `yaml:"permissions,omitempty" json:"permissions,omitempty" desc:"File permissions" default:"0700"`
}

//nolint:lll
type Bootstrap struct {
	SSH                           ssh.SSHClient              `yaml:"ssh" json:"ssh" desc:"Bootstrap SSH configuration (used during initial provisioning)"`
	DiskEncryptionKeys            []PlainFileOrDirToTransfer `yaml:"disk_encryption_keys" json:"disk_encryption_keys,omitempty" desc:"Keys are transferred to root dir on remote, which is the installer. If you want them to be transferred to disk of the final system, prefix path with '/mnt'" validate:"dive"`
	PostBootstrapHooks            []PostBootstrapHookCommand `yaml:"post_bootstrap_hooks" json:"post_bootstrap_hooks,omitempty" desc:"Commands to run after disko partitioning"`
	PostBootstrapInstallHooks     []PostBootstrapHookCommand `yaml:"post_bootstrap_install_hooks" json:"post_bootstrap_install_hooks,omitempty" desc:"Commands to run after nixos-install (before reboot)"`
	PostBootstrapProvisionedHooks []PostBootstrapHookCommand `yaml:"post_bootstrap_provisioned_hooks" json:"post_bootstrap_provisioned_hooks,omitempty" desc:"Commands to run after reboot (uses regular SSH)"`
	Kexec                         KexecConfig                `yaml:"kexec" json:"kexec" desc:"Kexec configuration for bootstrapping non-NixOS machines or reinstalling a live NixOS installation"`
	DisableDisko                  bool                       `yaml:"disable_disko" json:"disable_disko,omitempty" desc:"Disables building, transfer and execution of disko tool"`
	DisableAutomaticReboot        bool                       `yaml:"disable_automatic_reboot" json:"disable_automatic_reboot,omitempty" desc:"Disable automatic reboot after nixos-install (useful for manual inspection or custom reboot handling)"`
	ForceBootstrap                bool                       `yaml:"force_bootstrap" json:"force_bootstrap,omitempty" desc:"Force bootstrap even if machine is already NixOS (requires allow_destructive_actions)" validate:"required_if=ForceBootstrapKexec true"`
	ForceBootstrapKexec           bool                       `yaml:"force_bootstrap_kexec" json:"force_bootstrap_kexec,omitempty" desc:"Force kexec method even if already in NixOS installer (requires force_bootstrap and allow_destructive_actions)"`
	AllowDestructiveActions       bool                       `yaml:"allow_destructive_actions" json:"allow_destructive_actions,omitempty" desc:"Allow destructive bootstrap actions (required for force_bootstrap and force_bootstrap_kexec)" validate:"required_if=ForceBootstrap true,required_if=ForceBootstrapKexec true"`
}

//nolint:lll
type KexecConfig struct {
	Image      KexecImage `yaml:"image" json:"image,omitempty" desc:"URL or path to kexec tarball for bootstrapping non-NixOS machines" validate:"omitempty,url|filepath" default:"https://github.com/nix-community/nixos-images/releases/latest/download/nixos-kexec-installer-noninteractive-<arch>-linux.tar.gz"`
	ExtraFlags []string   `yaml:"extra_flags" json:"extra_flags,omitempty" desc:"Extra flags to pass to kexec (e.g. '--no-sync')"`
	SSHPort    uint16     `yaml:"ssh_port,omitempty" json:"ssh_port,omitempty" desc:"SSH port for kexec installer" default:"22"`
}

type PostBootstrapHookCommand string

const PostBootstrapHookWaitForOnline PostBootstrapHookCommand = "waitForOnline"
const PostBootstrapHookWaitForOffline PostBootstrapHookCommand = "waitForOffline"

func New() *Attributes {
	return &Attributes{}
}

func (a *Attributes) Init(name string, parentAttr *Attributes) error {
	err := a.passAttributesInto(name, parentAttr)
	if err != nil {
		return err
	}

	return nil
}

// InitSSH initializes SSH configuration for this machine.
// SSH config resolution errors (e.g., missing ~/.ssh/config) are logged as warnings
// and do not prevent initialization — the SSH client retains alias hostname with defaults.
func (a *Attributes) InitSSH(localMachineHostname string, nixInfo nixver.Info) error {
	err := a.SSH.Init(a.Name, localMachineHostname, nixInfo)
	if err != nil {
		return errors.Wrapf(err, "%s", a.Xpath.String())
	}

	if a.Bootstrap.SSH.IsInitialized() {
		err = a.Bootstrap.SSH.Init(a.Name, localMachineHostname, nixInfo)
		if err != nil {
			return errors.Wrapf(err, "%s", a.Xpath.String())
		}
	}

	return nil
}

// passAttributesInto merges parent attributes into child ones, without overriding.
// For this to work poperly all attributes have to be non-pointers (except leafs,
// as mergo does not merge individual fields of pointer types, just whole pointer)
// Has to be run before rest of the Init.
func (a *Attributes) passAttributesInto(name string, parentAttr *Attributes) error {
	err := mergo.Merge(a, parentAttr, mergo.WithAppendSlice)
	if err != nil {
		return errors.Wrap(err, "failed to merge attributes")
	}

	// Custom set/merge
	if name != "" {
		a.Name = name
		a.Tags = append(a.Tags, name)
		a.Xpath = parentAttr.Xpath.NewXpathWithAppend(name)
	}

	// Pre-compute phase xpaths for this entity — used by TUI rendering.
	a.PhaseXpaths = make(map[phase.Phase]xpath.Xpath, len(phase.PhaseRegistry))

	for _, pm := range phase.PhaseRegistry {
		a.PhaseXpaths[pm.Phase] = a.Xpath.NewXpathWithAppend(pm.Phase.String())
	}

	return nil
}
