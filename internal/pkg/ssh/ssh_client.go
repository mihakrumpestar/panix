package ssh

import (
	"strconv"
	"strings"
)

//nolint:lll,recvcheck
type SSHClient struct {
	// Machine key is ssh alias if hostname is not set
	Hostname              string      `yaml:"hostname" json:"hostname,omitempty" desc:"SSH hostname or IP address"`
	Port                  SSHPort     `yaml:"port" json:"port,omitempty" desc:"SSH port number" default:"22"`
	Username              SSHUsername `yaml:"username" json:"username,omitempty" desc:"SSH username" default:"root"`
	IdentityFile          string      `yaml:"identity_file" json:"identity_file,omitempty" validate:"omitempty,filepath" desc:"Path to SSH private key"`
	StrictKeyChecking     bool        `yaml:"strict_key_checking" json:"strict_key_checking,omitempty" desc:"Enable strict host key checking (default: false for bootstrap SSH, true for regular SSH)"`
	DisableAutoAddHostKey bool        `yaml:"disable_auto_add_host_key" json:"disable_auto_add_host_key,omitempty" desc:"Disable automatically adding host key to known_hosts on first connection (default: true for bootstrap SSH, false for regular SSH)"`
	ExtraFlags            []string    `yaml:"extra_flags" json:"extra_flags,omitempty" desc:"Extra flags passed to ssh (e.g. '-o', 'StrictHostKeyChecking=no')"`

	isLocal         bool
	hostnameIsAlias bool
}

func (sC *SSHClient) Init(sshConfig *SSHConfig, machineName, localMachine string) error {
	// Use machineName as Hostname if Hostname is empty (indicates SSH config alias usage)
	if sC.Hostname == "" {
		sC.hostnameIsAlias = true
		sC.Hostname = machineName
	}

	// Even if it is alias we need to get port info for certain tasks
	if sC.hostnameIsAlias {
		err := sshConfig.RetrieveFullParamsFromSSHConfig(sC)
		if err != nil {
			return err
		}
	}

	// Check if machine is local after hostname is fully resolved (from SSH config if alias)
	sC.isLocal = sC.Hostname == localMachine

	return nil
}

func (sC SSHClient) IsInitialized() bool {
	return sC.Hostname != ""
}

func (sC SSHClient) IsLocal() bool {
	return sC.isLocal
}

func (sC *SSHClient) PortString() string {
	return strconv.Itoa(int(sC.Port.Get()))
}

func (sC *SSHClient) MaybeSSHCommandArguments() []string {
	var sshArgs []string

	if !sC.hostnameIsAlias {
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

	sshArgs = append(sshArgs, sC.ExtraFlags...)

	return sshArgs
}

func (sC *SSHClient) MaybeSSHEnvOpts() []string {
	sshArgs := sC.MaybeSSHCommandArguments()
	if len(sshArgs) == 0 {
		return nil
	}

	return []string{"NIX_SSHOPTS=" + strings.Join(sshArgs, " ")}
}
