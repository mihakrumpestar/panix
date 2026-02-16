package ssh

import (
	"errors"
	"fmt"
	"strings"
)

type SshClient struct {
	Hostname                 string `yaml:"hostname" desc:"SSH hostname or IP address"` // Hostname is alias if all other fileds are empty
	Port                     uint16 `yaml:"port" desc:"SSH port number" validate:"min=1"`
	Username                 string `yaml:"username" desc:"SSH username"`
	IdentityFile             string `yaml:"identity_file" desc:"Path to SSH private key" validate:"omitempty,filepath"`
	DisableStrictKeyChecking bool   `yaml:"disable_strict_key_checking" desc:"Disable strict host key checking"`
	// Internal
	IsLocal         bool `yaml:"-" validate:"-"`
	HostnameIsAlias bool `yaml:"-" validate:"-"`
}

func (sC *SshClient) Init(sshConfig *SshConfig, machineName, overrideLocalMachine string) error {
	if sC == nil {
		return errors.New("internal error: SshClient is nil")
	}

	if machineName == "" {
		return errors.New("machine name is empty")
	}

	// Use machineName as Hostname if Hostname is empty
	if sC.Hostname == "" {
		sC.HostnameIsAlias = true
		sC.Hostname = machineName
	}

	// Check if machine is local (eg. not remote) based on same Hostname
	sC.IsLocal = sC.Hostname == overrideLocalMachine

	if sC.HostnameIsAlias {
		if err := sshConfig.RetrieveFullParamsFromSshConfig(sC); err != nil {
			return err
		}
		return nil
	}

	if sC.Port == 0 {
		sC.Port = 22
	}
	if sC.Username == "" {
		sC.Username = "root"
	}

	return nil
}

func (sC *SshClient) MaybeSshCommandArguments() []string {
	if sC.HostnameIsAlias {
		return nil
	}

	sshArgs := []string{"-p", fmt.Sprintf("%d", sC.Port), "-l", sC.Username}

	if sC.IdentityFile != "" {
		sshArgs = append(sshArgs, "-i", sC.IdentityFile, "-o", "IdentitiesOnly=yes")
	}

	if sC.DisableStrictKeyChecking {
		sshArgs = append(sshArgs, "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no")
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
