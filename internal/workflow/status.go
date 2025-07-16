package workflow

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/pkg/errors"
)

type StatusMetadatas struct {
	*MetadatasBase
	Statuses []*StatusMetadata
}

type StatusMetadata struct {
	*executioner.BaseMetadata
	Reachable         bool
	SSHConnectable    bool
	Bootstrapped      bool
	CurrentGeneration string
	LastDeployTime    string
}

// executeStatusPhase runs status checks in parallel across all machines
// and must complete fully before proceeding to next phase
func (w *WorkflowExecutor) ExecuteStatusPhase() error {
	sms := w.metadatas.StatusMetadatas
	if sms == nil {
		sms = &StatusMetadatas{}
	}

	if sms.Statuses == nil {
		sms.Statuses = make([]*StatusMetadata, 0)
	}

	if w.cfg.Global.Verbose {
		fmt.Println("Executing status phase across all machines")
	}

	err := w.forEachFlakeConfiguration(sms.MetadatasBase, func(wp *WorkflowExecutorForConfigurationAndMachine, bm *executioner.BaseMetadata, configuration *config.Configuration) error {
		statuses := sms.Statuses

		sm := &StatusMetadata{}
		statuses = append(statuses, sm)

		return w.forEachConfigurationMachine(configuration, sms.MetadatasBase, bm, func(wp *WorkflowExecutorForConfigurationAndMachine, bm *executioner.BaseMetadata, machine *config.Machine) error {
			return wp.executeStatusPhaseMachineStreams(sm, machine, w.hook.OnUpdateHook())
		})
	})

	return err
}

func (w *WorkflowExecutorForConfigurationAndMachine) executeStatusPhaseMachineStreams(sm *StatusMetadata, machine *config.Machine, onUpdateHook func()) error {
	exc := executioner.New(w.ctx, sm.BaseMetadata, onUpdateHook, w.cfg, machine)

	// TCP check
	err := exc.PingStream(
		func(bm *executioner.BaseMetadata, err error) error {
			return fmt.Errorf("machine unreachable: %w", err)
		},
		func(bm *executioner.BaseMetadata) {
			sm.Reachable = true
		})
	if err != nil {
		return err
	}

	// SSH connect
	err = exc.Exec(
		func(bm *executioner.BaseMetadata, err error) error {
			return errors.Wrapf(err, "ssh test failed: %s", bm.CommandOutputs[len(bm.CommandOutputs)-1].Stderr.String())
		},
		func(bm *executioner.BaseMetadata) {
			sm.SSHConnectable = true
		}, "sh", "-c", "exit 0")
	if err != nil {
		return err
	}

	// Run bootstrap detection
	err = exc.Exec(
		nil,
		func(bm *executioner.BaseMetadata) {
			sm.Bootstrapped = true
		}, "sh", "-c", "test -e /run/current-system")
	if err != nil {
		return nil // just not bootstrapped, not really an error
	}

	// Get current generation
	err = exc.Exec(
		nil,
		func(bm *executioner.BaseMetadata) {
			sm.CurrentGeneration = strings.TrimSpace(bm.CommandOutputs[len(bm.CommandOutputs)-1].Stdout.String())
		}, "sh", "-c", "nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
	if err != nil {
		return err
	}

	// Get last deploy time
	err = exc.Exec(
		nil,
		func(bm *executioner.BaseMetadata) {
			sm.LastDeployTime = strings.TrimSpace(bm.CommandOutputs[len(bm.CommandOutputs)-1].Stdout.String())
		}, "sh", "-c", "stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
	if err != nil {
		return err
	}

	return nil
}

func (w *WorkflowExecutor) PrintStatusPhaseMachineTable() {
	if w.cfg.Global.DryRun {
		if w.cfg.Global.Verbose {
			fmt.Println("No status table when dry-run option is enabled")
		}
		return
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("INDEX", "ICON", "FLAKE", "CONFIGURATION", "MACHINE" /* "HOST", */, "STATUS", "GENERATION", "LAST_DEPLOY", "ERROR")

	for i, sm := range w.metadatas.StatusMetadatas.Statuses {
		err := ""
		if sm.Error != nil {
			err = sm.Error.Error()
		}

		t.Row(
			fmt.Sprintf("%d", i),
			sm.getStatusIcon(),
			sm.FlakeName,
			sm.ConfigurationName,
			sm.MachineName.String(),
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
	if !s.EndTime.IsZero() {
		return spinner.New().View()
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
