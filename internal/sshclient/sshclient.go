package sshclient

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/kevinburke/ssh_config"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/olekukonko/tablewriter"
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

type CheckDepth int

const (
	CheckMinimal CheckDepth = iota // Only reachability and SSH connection
	CheckFull                      // Full status including generation and deploy time
)

type MachineStatus struct {
	Machine           config.MachineConfig
	Reachable         bool
	SSHConnectable    bool
	Bootstrapped      bool
	AllOk             bool
	CurrentGeneration string
	LastDeployTime    string
	Error             error
}

// CheckHost performs TCP reachability, SSH login, and bootstrap detection
// depth parameter controls how much information to gather
func (c *SshClients) CheckHost(machineConfig config.MachineConfig, depth CheckDepth) (*MachineStatus, error) {
	status := &MachineStatus{
		Machine:           machineConfig,
		Reachable:         false,
		SSHConnectable:    false,
		Bootstrapped:      false,
		AllOk:             false,
		CurrentGeneration: "unknown",
		LastDeployTime:    "unknown",
	}

	sshClient, err := c.GetMachine(machineConfig)
	if err != nil {
		status.Error = err
		return status, err
	}

	// TCP check
	if _, err := net.DialTimeout("tcp", sshClient.params.address, config.C.Global.Timeout); err != nil {
		status.Error = fmt.Errorf("%s unreachable: %w", sshClient.params.address, err)
		return status, status.Error
	}
	status.Reachable = true

	// SSH connect
	client, err := sshClient.Client()
	if err != nil {
		status.Error = fmt.Errorf("ssh failed: %w", err)
		return status, status.Error
	}
	defer client.Close()
	status.SSHConnectable = true

	// Run bootstrap detection
	sess, err := client.NewSession()
	if err != nil {
		status.Error = fmt.Errorf("session failed: %w", err)
		return status, status.Error
	}
	defer sess.Close()

	if err := sess.Run("test -e /run/current-system"); err != nil {
		return status, nil // not bootstrapped
	}
	status.Bootstrapped = true

	// If full check requested, gather additional information
	if depth == CheckFull {
		if generation, err := c.getCurrentGeneration(client); err == nil {
			status.CurrentGeneration = generation
		}

		if deployTime, err := c.getLastDeployTime(client); err == nil {
			status.LastDeployTime = deployTime
		}
	}

	status.AllOk = true
	return status, nil
}

func (c *SshClients) GetMachineStatus(machineConfig config.MachineConfig) *MachineStatus {
	status, _ := c.CheckHost(machineConfig, CheckFull)
	return status
}

func (c *SshClients) getCurrentGeneration(client *ssh.Client) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	output, err := sess.Output("nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(output)), nil
}

func (c *SshClients) getLastDeployTime(client *ssh.Client) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	output, err := sess.Output("stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(output)), nil
}

func (c *SshClients) GetAllMachineStatuses(machines []config.MachineConfig) []*MachineStatus {
	concurrency := config.C.Global.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	jobs := make(chan config.MachineConfig, len(machines))
	results := make(chan *MachineStatus, len(machines))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for machine := range jobs {
				status := c.GetMachineStatus(machine)
				results <- status
			}
		}()
	}

	for _, machine := range machines {
		jobs <- machine
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var statuses []*MachineStatus
	for status := range results {
		statuses = append(statuses, status)
	}

	return statuses
}

func (s *MachineStatus) GetStatusIcon() string {
	if s.Error != nil {
		return "❌"
	}
	if !s.Reachable {
		return "🔴"
	}
	if !s.SSHConnectable {
		return "🟡"
	}
	if !s.Bootstrapped {
		return "🟠"
	}
	return "✅"
}

func (s *MachineStatus) GetStatusText() string {
	if s.Error != nil {
		return fmt.Sprintf("ERROR: %s", s.Error.Error())
	}
	if !s.Reachable {
		return "UNREACHABLE"
	}
	if !s.SSHConnectable {
		return "SSH_FAILED"
	}
	if !s.Bootstrapped {
		return "NOT_BOOTSTRAPPED"
	}
	return "OK"
}

func PrintStatusTable(statuses []*MachineStatus) {
	if len(statuses) == 0 {
		fmt.Println("No machines to display")
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("", "MACHINE", "HOST", "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	for _, status := range statuses {
		errorMsg := ""
		if status.Error != nil {
			errorMsg = status.Error.Error()
			if len(errorMsg) > 50 {
				errorMsg = errorMsg[:47] + "..."
			}
		}

		table.Append(
			status.GetStatusIcon(),
			status.Machine.Name,
			status.Machine.Ssh.Host,
			status.GetStatusText(),
			status.CurrentGeneration,
			status.LastDeployTime,
			errorMsg,
		)
	}

	table.Render()
}
