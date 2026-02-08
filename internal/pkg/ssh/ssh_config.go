package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/kevinburke/ssh_config"
)

type SshConfig struct {
	sc *ssh_config.Config
}

var (
	cachedSshConfig *SshConfig
	cachedError     error
	once            sync.Once
)

func GetCachedSshConfig() (*SshConfig, error) {
	once.Do(func() {
		// Load SSH config
		home := os.Getenv("HOME")
		cfgPath := filepath.Join(home, ".ssh", "config")
		sshCfgRaw, err := os.Open(cfgPath)
		if err != nil {
			cachedError = fmt.Errorf("failed to open SSH config: %w", err)
			return
		}
		defer sshCfgRaw.Close()

		sshCfg, err := ssh_config.Decode(sshCfgRaw)
		if err != nil {
			cachedError = fmt.Errorf("failed to parse SSH config: %w", err)
			return
		}

		cachedSshConfig = &SshConfig{sshCfg}
	})

	return cachedSshConfig, cachedError
}

func (sc *SshConfig) RetrieveFullParamsFromSshConfig(sshClient *SshClient) error {
	alias := sshClient.Hostname

	hostname, err := sc.sc.Get(alias, "HostName")
	if err != nil {
		return fmt.Errorf("failed to get HostName for alias %q: %w", alias, err)
	}
	if hostname == "" {
		return fmt.Errorf("ssh config for alias %q has empty or missing HostName", alias)
	}
	sshClient.Hostname = hostname

	portRaw, _ := sc.sc.Get(alias, "Port")
	if portRaw != "" {
		port64, err := strconv.ParseUint(portRaw, 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse Port for alias %q: %w", alias, err)
		}
		sshClient.Port = uint16(port64)
	} else {
		sshClient.Port = 22
	}

	username, _ := sc.sc.Get(alias, "User")
	if username != "" {
		sshClient.Username = username
	} else {
		sshClient.Username = "root"
	}

	return nil
}
