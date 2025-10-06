package ssh

import (
	"fmt"
)

type SshClient struct {
	Hostname     string `yaml:"hostname"` // Hostname is alias if all other fileds are empty
	Port         uint16 `yaml:"port"`
	Username     string `yaml:"username"`
	IdentityFile string `yaml:"identityFile"` // Path to private/public key
	// Internal
	IsLocal         bool
	HostnameIsAlias bool
}

func (sC *SshClient) Init(sshConfig *SshConfig, machineName, overrideLocalMachine string) error {
	if sC == nil {
		return nil
	}

	// Use machineName as Hostname if Hostname is empty
	if sC.Hostname == "" {
		sC.HostnameIsAlias = true
		sC.Hostname = machineName
	}

	// Check if machine is local (eg. not remote) based on same Hostname
	sC.IsLocal = sC.Hostname == overrideLocalMachine

	// Hostname is alias
	if sC.HostnameIsAlias {

		err := sshConfig.RetriveFullParamsFromSshConfig(sC)
		if err != nil {
			return err
		}

		return nil
	}

	// Hostname is not alias
	switch {
	case sC.Hostname == "":
		return fmt.Errorf("hostname can't be empty")
	case sC.Port == 0:
		return fmt.Errorf("port can't be empty")
	case sC.Username == "":
		return fmt.Errorf("username can't be empty")
	}

	return nil
}

func (sC *SshClient) Url() string {
	return fmt.Sprintf("ssh//%s@%s:%d", sC.Username, sC.Hostname, sC.Port)
}
