package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/kevinburke/ssh_config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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
		// Load SSH config using secure home directory resolution
		home, err := os.UserHomeDir()
		if err != nil {
			cachedError = errors.Wrap(err, "failed to get user home directory")
			return
		}

		cfgPath := filepath.Join(home, ".ssh", "config")

		sshCfgRaw, err := os.Open(cfgPath)
		if err != nil {
			cachedError = errors.Wrap(err, "failed to open SSH config")
			return
		}
		defer sshCfgRaw.Close()

		sshCfg, err := ssh_config.Decode(sshCfgRaw)
		if err != nil {
			cachedError = errors.Wrap(err, "failed to parse SSH config")
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

	portRaw, err := sc.sc.Get(alias, "Port")
	if err != nil {
		log.Warn().Err(err).Str("alias", alias).Msg("failed to read Port from SSH config")
	}

	if portRaw != "" {
		port64, err := strconv.ParseUint(portRaw, 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse Port for alias %q: %w", alias, err)
		}

		sshClient.Port = uint16(port64)
	} else {
		sshClient.Port = DefaultSSHPort
	}

	username, err := sc.sc.Get(alias, "User")
	if err != nil {
		log.Warn().Err(err).Str("alias", alias).Msg("failed to read User from SSH config")
	}

	if username != "" {
		sshClient.Username = username
	} else {
		sshClient.Username = DefaultSSHUsername
	}

	return nil
}
