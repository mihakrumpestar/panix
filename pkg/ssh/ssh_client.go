package ssh

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	SSHDefaultUsername = "root"
	SSHDefaultPort     = 22
)

//nolint:lll,recvcheck
type SSHClient struct {
	// Machine key is ssh alias if hostname is not set
	Hostname                 string         `yaml:"hostname,omitempty" json:"hostname,omitempty" desc:"SSH hostname or IP address" validate:"omitempty,hostname_rfc1123"`
	Port                     uint16         `yaml:"port,omitempty" json:"port,omitempty" desc:"SSH port number" default:"22" validate:"omitempty,port"`
	Username                 string         `yaml:"username,omitempty" json:"username,omitempty" desc:"SSH username" default:"root"`
	IdentityFile             string         `yaml:"identity_file,omitempty" json:"identity_file,omitempty" validate:"omitempty,filepath" desc:"Path to SSH private key"`
	DisableStrictKeyChecking bool           `yaml:"disable_strict_key_checking,omitempty" json:"disable_strict_key_checking,omitempty" desc:"Disable strict host key checking (default: false)"`
	DisableAutoAddHostKey    bool           `yaml:"disable_auto_add_host_key,omitempty" json:"disable_auto_add_host_key,omitempty" desc:"Disable automatically adding host key to known_hosts on first connection (default: false)"`
	KnownHostsFile           KnownHostsFile `yaml:"known_hosts_file,omitempty" json:"known_hosts_file,omitempty" validate:"omitempty,filepath" desc:"Path to known_hosts file for SSH host key verification (default: user's ~/.ssh/known_hosts, bootstrap SSH uses a temporary file)"`
	ExtraFlags               []string       `yaml:"extra_flags,omitempty" json:"extra_flags,omitempty" desc:"Extra flags passed to ssh (e.g. '-o', 'StrictHostKeyChecking=no')"`

	isLocal         bool
	hostnameIsAlias bool
	alias           string
}

func (sC *SSHClient) Init(sshConfig *SSHConfig, machineName, localMachine string) error {
	var err error

	sC.IdentityFile, err = resolveIdentityFile(sC.IdentityFile)
	if err != nil {
		return err
	}

	sC.resolveHostname(machineName)

	// Determine if machine is local before accessing SSH config.
	// Local machines don't need SSH at all, so skip SSH config resolution.
	sC.isLocal = sC.Hostname == localMachine

	if sC.hostnameIsAlias && !sC.isLocal {
		if sshConfig == nil {
			sshConfig, err = GetCachedSSHConfig()
			if err != nil {
				return errors.Wrap(err, "ssh config required for alias resolution but could not be loaded")
			}
		}

		err = sshConfig.RetrieveFullParamsFromSSHConfig(sC)
		if err != nil {
			return err
		}
	}

	sC.applyDefaults()

	return nil
}

func (sC SSHClient) IsInitialized() bool {
	return sC.Hostname != ""
}

// SSHTarget returns the hostname or alias to use as the SSH connection target.
// When the hostname is an SSH config alias, returns the alias (SSH config resolves it).
// Otherwise returns the resolved hostname (IP address).
func (sC SSHClient) SSHTarget() string {
	if sC.alias != "" {
		return sC.alias
	}

	return sC.Hostname
}

func (sC SSHClient) IsLocal() bool {
	return sC.isLocal
}

func (sC SSHClient) PortString() string {
	return strconv.Itoa(int(sC.Port))
}

func (sC SSHClient) HostPortString() string {
	return net.JoinHostPort(sC.Hostname, sC.PortString())
}

// MaybeSSHCommandArguments returns SSH CLI arguments for use by panix's executioner.
// These are NOT suitable for NIX_SSHOPTS — use MaybeNixSSHOpts() for that.
func (sC *SSHClient) MaybeSSHCommandArguments() []string {
	var sshArgs []string

	if !sC.hostnameIsAlias {
		sshArgs = append(sshArgs, "-l", sC.Username, "-p", sC.PortString())

		if sC.IdentityFile != "" {
			sshArgs = append(sshArgs, "-i", sC.IdentityFile, "-o", "IdentitiesOnly=yes")
		}
	}

	sshArgs = append(sshArgs, sC.hostKeySSHArgs()...)
	sshArgs = append(sshArgs, sC.ExtraFlags...)

	return sshArgs
}

// MaybeNixSSHOpts returns NIX_SSHOPTS environment variable entries for nix commands.
// Only includes host key verification settings and extra flags — identity files, ports,
// and usernames are handled via nix store URL params (NixStoreURL) or SSH config.
func (sC SSHClient) MaybeNixSSHOpts() []string {
	var sshArgs []string

	if sC.IdentityFile != "" {
		sshArgs = append(sshArgs, "-o", "IdentitiesOnly=yes")
	}

	sshArgs = append(sshArgs, sC.hostKeySSHArgs()...)
	sshArgs = append(sshArgs, sC.ExtraFlags...)

	if len(sshArgs) == 0 {
		return []string{"NIX_SSHOPTS="}
	}

	return []string{"NIX_SSHOPTS=" + strings.Join(sshArgs, " ")}
}

// NixStoreURL returns a nix store URL for this SSH client.
//
// When hostnameIsAlias=true, returns "ssh-ng://<alias>" (SSH config resolves everything else).
// When hostnameIsAlias=false, returns "ssh-ng://user@hostname:port[?ssh-key=<path>]"
func (sC SSHClient) NixStoreURL() string {
	if sC.hostnameIsAlias {
		return "ssh-ng://" + sC.SSHTarget()
	}

	url := "ssh-ng://" + sC.Username + "@" + sC.Hostname + ":" + sC.PortString()

	if sC.IdentityFile != "" {
		url += "?ssh-key=" + sC.IdentityFile
	}

	return url
}

// NixStoreURLWithParams returns NixStoreURL() with additional URL query parameters appended.
// Handles the correct separator (& vs ?) depending on whether the URL already has query params.
// e.g., NixStoreURLWithParams("remote-store=local", "root=/mnt").
func (sC SSHClient) NixStoreURLWithParams(params ...string) string {
	url := sC.NixStoreURL()

	var parts []string

	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}

	for _, p := range params {
		parts = append(parts, separator+p)
		separator = "&"
	}

	return url + strings.Join(parts, "")
}

// resolveHostname sets the hostname from machineName. If all minimal SSH config fields are
// unset (indicating SSH config alias usage), machineName becomes the alias and hostnameIsAlias
// is set. Otherwise, machineName is used as the hostname directly.
func (sC *SSHClient) resolveHostname(machineName string) {
	if sC.Hostname == "" && sC.Port == 0 && sC.Username == "" && sC.IdentityFile == "" {
		sC.Hostname = machineName
		sC.hostnameIsAlias = true
		sC.alias = machineName
	} else if sC.Hostname == "" {
		sC.Hostname = machineName
	}
}

// applyDefaults sets default values for username and port if not explicitly configured.
func (sC *SSHClient) applyDefaults() {
	if sC.Username == "" {
		sC.Username = SSHDefaultUsername
	}

	if sC.Port == 0 {
		sC.Port = SSHDefaultPort
	}
}

// hostKeySSHArgs returns SSH arguments for host key verification settings.
func (sC SSHClient) hostKeySSHArgs() []string {
	var sshArgs []string

	switch {
	case sC.DisableStrictKeyChecking:
		sshArgs = append(sshArgs, "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no")
	case sC.KnownHostsFile != "":
		sshArgs = append(sshArgs, "-o", "UserKnownHostsFile="+string(sC.KnownHostsFile))
		if !sC.DisableAutoAddHostKey {
			sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
		}
	case !sC.DisableAutoAddHostKey:
		sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
	}

	return sshArgs
}

func resolveIdentityFile(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.Wrap(err, "failed to resolve home directory for identity file")
		}

		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve absolute path for identity file")
	}

	return abs, nil
}
