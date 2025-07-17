package config

import (
	"fmt"
	"net/url"
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

func (sc *SshConfig) RetriveFullParamsFromSshConfig(machineName url.URL) (*url.URL, error) {
	host, err := sc.sc.Get(machineName.Hostname(), "HostName")
	if err != nil {
		return nil, err
	}

	user, err := sc.sc.Get(machineName.Host, "User")
	if err != nil {
		return nil, err
	}

	portRaw, err := sc.sc.Get(machineName.Host, "Port")
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return nil, err
	}

	urlString := fmt.Sprintf("ssh://%s@%s:%d", user, host, port)

	return url.Parse(urlString)
}
