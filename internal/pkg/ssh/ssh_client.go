package ssh

import (
	"strconv"
	"strings"
)

type SSHClient struct {
	Hostname              string      `yaml:"hostname" json:"hostname" desc:"SSH hostname or IP address"` // Hostname is alias if all other fileds are empty
	Port                  SSHPort     `yaml:"port" json:"port" desc:"SSH port number" default:"22"`
	Username              SSHUsername `yaml:"username" json:"username" desc:"SSH username" default:"root"`
	IdentityFile          string      `yaml:"identity_file" json:"identity_file" validate:"omitempty,filepath" desc:"Path to SSH private key"`
	StrictKeyChecking     bool        `yaml:"strict_key_checking" json:"strict_key_checking,omitempty" desc:"Enable strict host key checking (default: false for bootstrap SSH, true for regular SSH)"`                                                      //nolint:lll
	DisableAutoAddHostKey bool        `yaml:"disable_auto_add_host_key" json:"disable_auto_add_host_key,omitempty" desc:"Disable automatically adding host key to known_hosts on first connection (default: true for bootstrap SSH, false for regular SSH)"` //nolint:lll
	ExtraFlags            []string    `yaml:"extra_flags" json:"extra_flags,omitempty" desc:"Extra flags passed to ssh (e.g. '-o', 'StrictHostKeyChecking=no')"`                                                                                             //nolint:lll
	// Internal - computed during Init(), should never inherit from parent
	IsLocal         bool `yaml:"-" json:"-" mergo:"-"`
	HostnameIsAlias bool `yaml:"-" json:"-" mergo:"-"`
}

func (sC *SSHClient) Init(sshConfig *SSHConfig, machineName, localMachine string) error {
	// Use machineName as Hostname if Hostname is empty (indicates SSH config alias usage)
	if sC.Hostname == "" {
		sC.HostnameIsAlias = true
		sC.Hostname = machineName
	}

	if sC.HostnameIsAlias {
		err := sshConfig.RetrieveFullParamsFromSSHConfig(sC)
		if err != nil {
			return err
		}
	}

	// Check if machine is local after hostname is fully resolved (from SSH config if alias)
	sC.IsLocal = sC.Hostname == localMachine

	if sC.HostnameIsAlias {
		return nil
	}

	return nil
}

func (sC *SSHClient) PortString() string {
	return strconv.Itoa(int(sC.Port.Get()))
}

func (sC *SSHClient) MaybeSSHCommandArguments() []string {
	var sshArgs []string

	if !sC.HostnameIsAlias {
		sshArgs = []string{"-p", sC.PortString(), "-l", sC.Username.Get()}

		if sC.IdentityFile != "" {
			sshArgs = append(sshArgs, "-i", sC.IdentityFile, "-o", "IdentitiesOnly=yes")
		}
	}

	if !sC.StrictKeyChecking {
		sshArgs = append(sshArgs, "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no")
	} else if !sC.DisableAutoAddHostKey {
		sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
	}

	for _, extraFlag := range sC.ExtraFlags {
		sshArgs = append(sshArgs, extraFlag)
	}

	return sshArgs
}

func (sC *SSHClient) MaybeSSHEnvOpts() []string {
	sshArgs := sC.MaybeSSHCommandArguments()
	if len(sshArgs) == 0 {
		return nil
	}

	return []string{"NIX_SSHOPTS=" + strings.Join(sshArgs, " ")}
}
