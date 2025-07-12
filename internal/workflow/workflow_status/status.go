package workflow_status

import (
	"context"
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

type CheckDepth int

const (
	CheckMinimal CheckDepth = iota // Only reachability and SSH connection
	CheckFull                      // Full status including generation and deploy time
)

type MachineStatus struct {
	Name              string
	Machine           config.Machine
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
func CheckHost(ctx context.Context, machineName string, machineConfig config.Machine, depth CheckDepth) (status *MachineStatus) {
	status = &MachineStatus{
		Name:              machineName,
		Machine:           machineConfig,
		Reachable:         false,
		SSHConnectable:    false,
		Bootstrapped:      false,
		AllOk:             false,
		CurrentGeneration: "unknown",
		LastDeployTime:    "unknown",
	}

	var exc *executioner.Executioner
	exc, status.Error = executioner.New(ctx, false, machineConfig.Ssh)
	if status.Error != nil {
		return
	}

	// TCP check
	/*
		if _, err := net.DialTimeout("tcp", machineConfig.Ssh.Alias, config.C.Global.Timeout); err != nil {

			return status, fmt.Errorf("%s unreachable: %w", sshClient.Params.Address, err)
		}
		status.Reachable = true
	*/

	// SSH connect
	var output executioner.ExecutionerOutput
	output, status.Error = exc.Exec("exit 0")
	if status.Error != nil {
		status.Error = fmt.Errorf("ssh failed: %w\n%s", status.Error, &output.Stderr)
		return
	}
	status.Reachable = true
	status.SSHConnectable = true

	// Run bootstrap detection
	_, status.Error = exc.Exec("test -e /run/current-system")
	if status.Error != nil {
		status.Error = nil // just not bootstrapped
		return
	}
	status.Bootstrapped = true

	// If full check requested, gather additional information
	if depth == CheckFull {
		// Get current generation
		output, status.Error = exc.Exec("nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
		if status.Error != nil {
			return
		}
		status.CurrentGeneration = strings.TrimSpace(output.Stdout.String())

		// Get last deploy time
		output, status.Error = exc.Exec("stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
		if status.Error != nil {
			return
		}
		status.LastDeployTime = strings.TrimSpace(output.Stdout.String())
	}

	status.AllOk = true
	return
}
