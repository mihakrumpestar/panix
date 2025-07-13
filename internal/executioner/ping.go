package executioner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kevinburke/ssh_config"
	"github.com/mihakrumpestar/panix/internal/config"
)

func (ex *Executioner) Ping() (ExecutionerOutput, error) {
	if ex.local { // If local, no need to ping
		return ExecutionerOutput{}, nil
	}

	sshArgs := []string{}

	sshArgs = append(sshArgs, "-zvw1")

	if ex.sshConfig.Alias != "" {
		sshConfig, err := retriveAliasParamsFromSshConfig(ex.sshConfig)
		if err != nil {
			return ExecutionerOutput{}, err
		}
		sshArgs = append(sshArgs, sshConfig.Host)
		sshArgs = append(sshArgs, fmt.Sprintf("%d", sshConfig.Port))
	} else {
		sshArgs = append(sshArgs, ex.sshConfig.Host)
		sshArgs = append(sshArgs, fmt.Sprintf("%d", ex.sshConfig.Port))
	}

	return ex.shell("nc", sshArgs...)
}

func retriveAliasParamsFromSshConfig(sshConfig *config.Ssh) (*config.Ssh, error) {
	result := config.Ssh{}

	sshCfg, err := loadSshConfig()
	if err != nil {
		return nil, err
	}

	result.Host, err = sshCfg.Get(sshConfig.Alias, "HostName")
	if err != nil {
		return nil, err
	}

	//fmt.Println("host:", sshConfig.Alias)

	portRaw, err := sshCfg.Get(sshConfig.Alias, "Port")
	if err != nil {
		return nil, err
	}

	result.Port, err = strconv.Atoi(portRaw)
	if err != nil {
		return nil, err
	}

	//fmt.Println("port:", sshClientParams.Port)

	return &result, nil
}

func loadSshConfig() (*ssh_config.Config, error) {
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

	return sshCfg, nil
}
