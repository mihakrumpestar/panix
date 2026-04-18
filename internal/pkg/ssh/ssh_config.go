package ssh

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/kevinburke/ssh_config"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var ErrSSHConfigMissingHostname = errors.New("ssh config has empty or missing HostName")

type SSHConfig struct {
	sshConfig *ssh_config.Config
}

var (
	cachedSSHConfig *SSHConfig
	cachedError     error //nolint:errname
	once            sync.Once
)

func GetCachedSSHConfig() (*SSHConfig, error) {
	once.Do(func() {
		// Load SSH config using secure home directory resolution
		home, err := os.UserHomeDir()
		if err != nil {
			cachedError = errors.Wrap(err, "failed to get user home directory")

			return
		}

		cfgPath := filepath.Join(home, ".ssh", "config")

		sshCfgRaw, err := os.Open(cfgPath) // #nosec G304 -- cfgPath is constructed from UserHomeDir() with known suffix
		if err != nil {
			cachedError = errors.Wrap(err, "failed to open SSH config")

			return
		}

		defer func() {
			err = sshCfgRaw.Close()
			if err != nil {
				log.Error().Err(err).Msg("failed to close SSH config file")
			}
		}()

		sshCfg, err := ssh_config.Decode(sshCfgRaw)
		if err != nil {
			cachedError = errors.Wrap(err, "failed to parse SSH config")

			return
		}

		cachedSSHConfig = &SSHConfig{sshCfg}
	})

	if cachedError != nil {
		return nil, cachedError
	}

	return cachedSSHConfig, nil
}

func (sc *SSHConfig) RetrieveFullParamsFromSSHConfig(sshClient *SSHClient) error {
	if sc == nil {
		return nil
	}

	alias := sshClient.Hostname

	hostname, err := sc.sshConfig.Get(alias, "HostName")
	if err != nil {
		return errors.Wrapf(err, "failed to get HostName for alias %q", alias)
	}

	if hostname == "" {
		return errors.Wrapf(ErrSSHConfigMissingHostname, "alias %q", alias)
	}

	sshClient.Hostname = hostname

	portRaw, err := sc.sshConfig.Get(alias, "Port")
	if err != nil {
		log.Warn().Err(err).Str("alias", alias).Msg("failed to read Port from SSH config")
	}

	if portRaw != "" {
		var port64 uint64

		port64, err = strconv.ParseUint(portRaw, 10, 16)
		if err != nil {
			return errors.Wrapf(err, "failed to parse Port for alias %q", alias)
		}

		sshClient.Port.Set(uint16(port64))
	}

	username, err := sc.sshConfig.Get(alias, "User")
	if err != nil {
		log.Warn().Err(err).Str("alias", alias).Msg("failed to read User from SSH config")
	}

	if username != "" {
		sshClient.Username.Set(username)
	}

	return nil
}
