package workflow_status

import (
	"bytes"
	"fmt"
	"net"

	"github.com/mihakrumpestar/panix/internal/clients"
	"github.com/mihakrumpestar/panix/internal/config"
)

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
func CheckHost(c *clients.SshClients, machineConfig config.MachineConfig, depth CheckDepth) (*MachineStatus, error) {
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
	if _, err := net.DialTimeout("tcp", sshClient.Params.Address, config.C.Global.Timeout); err != nil {
		status.Error = fmt.Errorf("%s unreachable: %w", sshClient.Params.Address, err)
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

	err = sess.Run("test -e /run/current-system")
	if err != nil {
		return status, nil // not bootstrapped
	}
	status.Bootstrapped = true

	// If full check requested, gather additional information
	if depth == CheckFull {
		// Get current generation
		output, err := sess.Output("nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
		if err != nil {
			return nil, err
		}
		status.CurrentGeneration = string(bytes.TrimSpace(output))

		// Get last deploy time
		output, err = sess.Output("stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
		if err != nil {
			return nil, err
		}
		status.LastDeployTime = string(bytes.TrimSpace(output))
	}

	status.AllOk = true
	return status, nil
}
