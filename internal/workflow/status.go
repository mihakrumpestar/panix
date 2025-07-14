package workflow

import (
	"fmt"
	"strings"

	"github.com/alitto/pond/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

type StatusMetadata struct {
	MetadataID
	Reachable         bool
	SSHConnectable    bool
	Bootstrapped      bool
	CurrentGeneration string
	LastDeployTime    string
	Error             error
}

// executeStatusPhase runs status checks in parallel across all machines
// and must complete fully before proceeding to next phase
func (w *WorkflowExecutor) executeStatusPhase(nextPhases []workflow_definition.WorkflowPhase) ([]StatusMetadata, error) {

	if w.cfg.Global.Verbose {
		fmt.Println("Executing status phase across all machines")
	}

	// Create a pool with limited concurrency
	pool := pond.NewResultPool[StatusMetadata](w.cfg.Global.Concurrency)
	group := pool.NewGroupContext(w.ctx)

	forAllMachines(w.cfg.Flakes, func(flakeName, configurationName, machineName string, machine *config.Machine) {
		group.SubmitErr(func() (StatusMetadata, error) {
			wp := WorkflowExecutorConfigurationMachine{w.ctx, &w.cfg.Global}
			result, err := wp.statusPhaseMachine(flakeName, configurationName, machineName, machine)
			result.Error = err

			return result, nil
		})
	})

	// Stop the pool and wait for all submitted tasks to complete
	results, _ := group.Wait()

	errors := make([]error, 0)
	for _, result := range results {
		if result.Error != nil {
			errors = append(errors, result.Error)
		}
	}

	if len(errors) > 0 && w.cfg.Global.RequireAllSuccess {
		return nil, fmt.Errorf("status phase failed: %v", errors)
	}

	w.PrintStatusPhaseMachineTable(results)

	if len(nextPhases) > 0 {
		return results, w.executePhase(nextPhases)
	}

	return results, nil
}

// CheckHost performs TCP reachability, SSH login, and bootstrap detection
// depth parameter controls how much information to gather
func (w *WorkflowExecutorConfigurationMachine) statusPhaseMachine(flakeName, configurationName, machineName string, machine *config.Machine) (sm StatusMetadata, err error) {
	sm.MetadataID = MetadataID{
		FlakeName:         flakeName,
		ConfigurationName: configurationName,
		MachineName:       machineName,
	}

	exc, err := executioner.New(w.ctx, w.cfg.DryRun, machine)
	if err != nil {
		return
	}

	// TCP check
	_, err = exc.Ping()
	if err != nil {
		err = fmt.Errorf("machine unreachable: %w", err)
		return
	}
	sm.Reachable = true

	// SSH connect
	output, err := exc.Exec("exit 0")
	if err != nil {
		err = fmt.Errorf("ssh failed: %w\n%s", err, output.Stderr.String())
		return
	}
	sm.SSHConnectable = true

	// Run bootstrap detection
	_, err = exc.Exec("test -e /run/current-system")
	if err != nil {
		return sm, nil // just not bootstrapped, not really an error
	}
	sm.Bootstrapped = true

	// Get current generation
	output, err = exc.Exec("nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
	if err != nil {
		return
	}
	sm.CurrentGeneration = strings.TrimSpace(output.Stdout.String())

	// Get last deploy time
	output, err = exc.Exec("stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
	if err != nil {
		return
	}
	sm.LastDeployTime = strings.TrimSpace(output.Stdout.String())

	return sm, nil
}

func (w *WorkflowExecutor) PrintStatusPhaseMachineTable(sms []StatusMetadata) {
	if w.cfg.Global.DryRun {
		if w.cfg.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("INDEX", "ICON", "FLAKE", "CONFIGURATION", "MACHINE" /* "HOST", */, "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	for i, sm := range sms {
		err := ""
		if sm.Error != nil {
			err = sm.Error.Error()
		}

		t.Row(
			fmt.Sprintf("%d", i),
			sm.getStatusIcon(),
			sm.FlakeName,
			sm.ConfigurationName,
			sm.MachineName,
			//machine.Ssh.Alias,
			sm.getStatusText(),
			sm.CurrentGeneration,
			sm.LastDeployTime,
			err,
		)
	}

	fmt.Println(t)
}

func (s *StatusMetadata) getStatusIcon() string {
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

func (s *StatusMetadata) getStatusText() string {
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
