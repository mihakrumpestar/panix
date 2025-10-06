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

func (sc *SshConfig) RetriveFullParamsFromSshConfig(sshClient *SshClient) error {
	alias := sshClient.Hostname
	var err error

	sshClient.Hostname, err = sc.sc.Get(alias, "HostName") // Required
	if err != nil {
		return err
	}

	if sshClient.Hostname == "" {
		return fmt.Errorf("does not have \"Hostname\" field (or it is empty) in ssh_config")
	}

	portRaw, err := sc.sc.Get(alias, "Port") // Not required
	if err == nil && portRaw != "" {
		var port64 uint64
		port64, err = strconv.ParseUint(portRaw, 10, 16)
		if err != nil {
			return err
		}

		sshClient.Port = uint16(port64)
	} else {
		sshClient.Port = 22 // Default
	}

	sshClient.Username, _ = sc.sc.Get(alias, "User") // Not required

	if sshClient.Username == "" {
		sshClient.Username = "root" // Default
	}

	return nil
}
