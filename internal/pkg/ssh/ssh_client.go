package ssh

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	DefaultSSHPort     uint16 = 22
	DefaultSSHUsername string = "root"
)

var (
	ErrSSHClientNil     = errors.New("internal error: SSHClient is nil")
	ErrMachineNameEmpty = errors.New("machine name is empty")
)

type SSHClient struct {
	Hostname              string `yaml:"hostname" json:"hostname" desc:"SSH hostname or IP address"` // Hostname is alias if all other fileds are empty
	Port                  uint16 `yaml:"port" json:"port,omitempty" desc:"SSH port number"`
	Username              string `yaml:"username" json:"username,omitempty" desc:"SSH username"`
	IdentityFile          string `yaml:"identity_file" json:"identity_file,omitempty" desc:"Path to SSH private key" validate:"omitempty,filepath"`
	StrictKeyChecking     bool   `yaml:"strict_key_checking" json:"strict_key_checking,omitempty" desc:"Enable strict host key checking (default: false for bootstrap SSH, true for regular SSH)"`                                                      //nolint:lll
	DisableAutoAddHostKey bool   `yaml:"disable_auto_add_host_key" json:"disable_auto_add_host_key,omitempty" desc:"Disable automatically adding host key to known_hosts on first connection (default: true for bootstrap SSH, false for regular SSH)"` //nolint:lll
	// Internal - computed during Init(), never inherit from parent
	IsLocal         bool `yaml:"-" json:"-" validate:"-" mergo:"-"`
	HostnameIsAlias bool `yaml:"-" json:"-" validate:"-" mergo:"-"`
}

func (sC *SSHClient) Init(sshConfig *SSHConfig, machineName, overrideLocalMachine string) error {
	if sC == nil {
		return ErrSSHClientNil
	}

	if machineName == "" {
		return ErrMachineNameEmpty
	}

	// Use machineName as Hostname if Hostname is empty (indicates SSH config alias usage)
	if sC.Hostname == "" {
		sC.HostnameIsAlias = true
		sC.Hostname = machineName
	}

	if sC.HostnameIsAlias {
		err := sshConfig.RetrieveFullParamsFromSSHConfig(sC)
		if err != nil {
			return err
		}
	}

	// Check if machine is local after hostname is fully resolved (from SSH config if alias)
	sC.IsLocal = sC.Hostname == overrideLocalMachine

	if sC.HostnameIsAlias {
		return nil
	}

	if sC.Port == 0 {
		sC.Port = DefaultSSHPort
	}

	if sC.Username == "" {
		sC.Username = DefaultSSHUsername
	}

	return nil
}

func (sC *SSHClient) PortString() string {
	return strconv.Itoa(int(sC.Port))
}

func (sC *SSHClient) MaybeSSHCommandArguments() []string {
	var sshArgs []string

	if !sC.HostnameIsAlias {
		sshArgs = []string{"-p", sC.PortString(), "-l", sC.Username}

		if sC.IdentityFile != "" {
			sshArgs = append(sshArgs, "-i", sC.IdentityFile, "-o", "IdentitiesOnly=yes")
		}
	}

	if !sC.StrictKeyChecking {
		sshArgs = append(sshArgs, "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no")
	} else if !sC.DisableAutoAddHostKey {
		sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
	}

	return sshArgs
}

func (sC *SSHClient) MaybeSSHEnvOpts() []string {
	sshArgs := sC.MaybeSSHCommandArguments()
	if len(sshArgs) == 0 {
		return nil
	}

	return []string{"NIX_SSHOPTS=" + strings.Join(sshArgs, " ")}
}
