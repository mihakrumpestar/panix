package ssh

import (
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultSSHPort     uint16 = 22
	DefaultSSHUsername string = "root"
)

type SshClient struct {
	Hostname              string `yaml:"hostname" desc:"SSH hostname or IP address"` // Hostname is alias if all other fileds are empty
	Port                  uint16 `yaml:"port" desc:"SSH port number"`
	Username              string `yaml:"username" desc:"SSH username"`
	IdentityFile          string `yaml:"identity_file" desc:"Path to SSH private key" validate:"omitempty,filepath"`
	StrictKeyChecking     bool   `yaml:"strict_key_checking" desc:"Enable strict host key checking (default: false for bootstrap SSH, true for regular SSH)"`
	DisableAutoAddHostKey bool   `yaml:"disable_auto_add_host_key" desc:"Disable automatically adding host key to known_hosts on first connection (default: true for bootstrap SSH, false for regular SSH)"`
	// Internal - computed during Init(), never inherit from parent
	IsLocal         bool `yaml:"-" json:"-" validate:"-" mergo:"-"`
	HostnameIsAlias bool `yaml:"-" json:"-" validate:"-" mergo:"-"`
}

func (sC *SshClient) Init(sshConfig *SshConfig, machineName, overrideLocalMachine string) error {
	if sC == nil {
		return errors.New("internal error: SshClient is nil")
	}

	if machineName == "" {
		return errors.New("machine name is empty")
	}

	// Use machineName as Hostname if Hostname is empty (indicates SSH config alias usage)
	if sC.Hostname == "" {
		sC.HostnameIsAlias = true
		sC.Hostname = machineName
	}

	if sC.HostnameIsAlias {
		if err := sshConfig.RetrieveFullParamsFromSshConfig(sC); err != nil {
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

func (sC *SshClient) MaybeSshCommandArguments() []string {
	var sshArgs []string

	if !sC.HostnameIsAlias {
		sshArgs = []string{"-p", fmt.Sprintf("%d", sC.Port), "-l", sC.Username}

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

func (sC *SshClient) MaybeSshEnvOpts() []string {
	sshArgs := sC.MaybeSshCommandArguments()
	if len(sshArgs) == 0 {
		return nil
	}

	return []string{"NIX_SSHOPTS=" + strings.Join(sshArgs, " ")}
}
