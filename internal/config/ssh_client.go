package config

import (
	"fmt"
	"os"
)

type SshClient struct {
	Hostname   string `yaml:"hostname"` // Hostname is alias if username and password are empty
	Port       uint16 `yaml:"port"`
	Username   string `yaml:"username"`
	Password   string `yaml:"username"`
	PrivateKey string `yaml:"privateKey"` // Path to private key
	PublicKey  string `yaml:"publicKey"`  // Path to public key

	IsLocal         bool
	HostnameIsAlias bool
}

func (sC *SshClient) Validate(sshConfig *SshConfig, machineName string) error {
	// Use machineName as Hostname if Hostname is empty
	if sC.Hostname == "" {
		sC.Hostname = machineName
	}

	// Check if machine is local (eg. not remote)
	localHostname, err := os.Hostname()
	if err == nil {
		sC.IsLocal = localHostname == sC.Hostname
	}

	// Hostname is alias
	if sC.Hostname != "" && sC.Port == 0 &&
		sC.Username == "" && sC.Password == "" &&
		sC.PrivateKey == "" && sC.PublicKey == "" {

		sC.HostnameIsAlias = true

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
	case sC.Password == "" && sC.PrivateKey == "" && sC.PublicKey == "":
		return fmt.Errorf("password, privateKey and publicKey can't be all empty")
	}

	return nil
}
