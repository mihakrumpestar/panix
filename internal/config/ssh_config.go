package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kevinburke/ssh_config"
)

type SshConfig struct {
	sc *ssh_config.Config
}

func LoadSshConfig() (*SshConfig, error) {
	// Load SSH config
	home := os.Getenv("HOME")
	cfgPath := filepath.Join(home, ".ssh", "config")
	sshCfgRaw, err := os.Open(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer sshCfgRaw.Close()

	sshCfg, err := ssh_config.Decode(sshCfgRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	return &SshConfig{sshCfg}, nil
}

func (sc *SshConfig) RetriveFullParamsFromSshConfig(sshClient *SshClient) error {
	alias := sshClient.Hostname
	var err error

	sshClient.Hostname, err = sc.sc.Get(alias, "HostName")
	if err != nil {
		return err
	}

	portRaw, err := sc.sc.Get(alias, "Port") // Not required
	if err == nil {
		var port64 uint64
		port64, err = strconv.ParseUint(portRaw, 10, 16)
		if err != nil {
			return err
		}

		sshClient.Port = uint16(port64)
	}

	sshClient.Username, _ = sc.sc.Get(alias, "User") // Not required

	sshClient.Password, _ = sc.sc.Get(alias, "Password") // Not required

	return nil
}
