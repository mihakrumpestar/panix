package executioner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
	"github.com/mihakrumpestar/panix/internal/config"
)

// PingStream runs “nc -zvw1 host port” (or no‐op if local) and
// streams back its stdout/stderr in ShellEvent.  Alias lookup
// errors are reported as a single event with Err set.
func (ex *Executioner) PingStream() <-chan ExecutionerOutput {
	ch := make(chan ExecutionerOutput)

	go func() {
		defer close(ch)

		// 1) local short‐circuit
		if ex.local {
			ch <- ExecutionerOutput{
				Command: "ping (local) → skipped",
				Stdout:  strings.Builder{},
				Stderr:  strings.Builder{},
			}
			return
		}

		// 2) build nc args
		args := []string{"-zvw1"}
		if ex.sshConfig.Alias != "" {
			aliasCfg, err := retriveAliasParamsFromSshConfig(ex.sshConfig)
			if err != nil {
				ch <- ExecutionerOutput{Error: err}
				return
			}
			args = append(args, aliasCfg.Host, fmt.Sprintf("%d", aliasCfg.Port))
		} else {
			args = append(
				args,
				ex.sshConfig.Host,
				fmt.Sprintf("%d", ex.sshConfig.Port),
			)
		}

		// 3) delegate to shellStream
		//
		// shellStream will send an initial event, then stream any
		// stdout/stderr lines, and finally one last event with Err
		// set on failure (or just a final success event).
		for evt := range ex.shellStream("nc", args...) {
			ch <- evt
		}
	}()

	return ch
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
