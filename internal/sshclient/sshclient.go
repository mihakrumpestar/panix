package sshclient

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kevinburke/ssh_config"
	"github.com/mihakrumpestar/panix/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client manages SSH config and authentication
type SshClients struct {
	machineSshClients map[string]*SshClient
}

type SshClient struct {
	params SshClientParams
	Client func() (*ssh.Client, error)
}

type SshClientParams struct {
	host    string
	port    int
	address string // combined host and port
	user    string
	auth    []ssh.AuthMethod
}

// New creates a Client with parsed SSH config and authentication methods
func New(cfg config.Config, machineConfigs []config.MachineConfig) (*SshClients, error) {
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

	// Prepare auth methods
	var auths []ssh.AuthMethod
	// private key if provided
	if cfg.Global.SshClientConfig.PrivateKey != "" {
		key, err := os.ReadFile(cfg.Global.SshClientConfig.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	// SSH agent if available
	var sshAgentSigners []ssh.Signer
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentClient := agent.NewClient(conn)

			sshAgentSigners, err = agentClient.Signers()
			if err != nil {
				return nil, err
			}

			auths = append(auths, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	machineSshClients := make(map[string]*SshClient)
	for _, machineConfig := range machineConfigs {
		id := machineConfig.Name + " " + machineConfig.FlakeName

		sshI := machineConfig.Ssh

		var sshClientParams = SshClientParams{
			host: sshI.Host,
			port: sshI.Port,
			user: sshI.User,
			auth: auths,
		}

		//privateKey := sshI.PrivateKey
		//publicKey := sshI.PublicKey

		sshConfigHost, _ := sshCfg.Get(sshI.Host, "HostName")
		if sshConfigHost != "" {
			sshClientParams.host = sshConfigHost

			fmt.Println("host:", sshClientParams.host)

			portRaw, err := sshCfg.Get(sshI.Host, "Port")
			if err != nil {
				return nil, err
			}

			sshClientParams.port, err = strconv.Atoi(portRaw)
			if err != nil {
				return nil, err
			}

			fmt.Println("port:", sshClientParams.port)

			sshClientParams.user, _ = sshCfg.Get(sshI.Host, "User")

			fmt.Println("user:", sshClientParams.user)

			identityFile, _ := sshCfg.Get(sshI.Host, "IdentityFile")
			if identityFile != "" {
				key, err := os.ReadFile(identityFile)
				if err != nil {
					return nil, fmt.Errorf("failed to read public key from '%s': %w", identityFile, err)
				}

				publicKeyParsed, err := ssh.ParsePublicKey(key)
				if err != nil {
					return nil, err
				}

				for _, sshAgentSigner := range sshAgentSigners {
					if bytes.Equal(sshAgentSigner.PublicKey().Marshal(), publicKeyParsed.Marshal()) {

						sshClientParams.auth = []ssh.AuthMethod{ssh.PublicKeys(sshAgentSigner)}
						break
					}
				}
			}
		}

		sshClientParams.address = net.JoinHostPort(sshClientParams.host, fmt.Sprintf("%d", sshClientParams.port))

		sshConfig := ssh.ClientConfig{
			User:            sshClientParams.user,
			Auth:            sshClientParams.auth,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         cfg.Global.Timeout,
		}

		machineSshClients[id] = &SshClient{
			params: sshClientParams,
			Client: func() (*ssh.Client, error) {
				return ssh.Dial("tcp", sshClientParams.address, &sshConfig)
			},
		}

	}

	return &SshClients{
		machineSshClients: machineSshClients,
	}, nil
}

func (c *SshClients) GetMachine(machineConfig config.MachineConfig) (*SshClient, error) {
	id := machineConfig.Name + " " + machineConfig.FlakeName

	sshClient := c.machineSshClients[id]
	if sshClient == nil {
		return nil, fmt.Errorf("machine with name %s and flake %s not in ssh config", machineConfig.Name, machineConfig.FlakeName)
	}

	return sshClient, nil
}

// CheckHost performs TCP reachability, SSH login, and bootstrap detection
// Returns true if machine is bootstrapped
func (c *SshClients) CheckHost(machineConfig config.MachineConfig) (bool, error) {

	sshClient, err := c.GetMachine(machineConfig)
	if err != nil {
		return false, err
	}

	// TCP check
	if _, err := net.DialTimeout("tcp", sshClient.params.address, config.C.Global.Timeout); err != nil {
		return false, fmt.Errorf("%s unreachable: %w", sshClient.params.address, err)
	}

	// SSH connect
	client, err := sshClient.Client()
	if err != nil {
		return false, fmt.Errorf("ssh failed: %w", err)
	}
	defer client.Close()

	// Run bootstrap detection
	sess, err := client.NewSession()
	if err != nil {
		return false, fmt.Errorf("session failed: %w", err)
	}
	defer sess.Close()

	if err := sess.Run("test -e /run/current-system"); err != nil {
		return false, nil // not bootstrapped
	}

	return true, nil
}
