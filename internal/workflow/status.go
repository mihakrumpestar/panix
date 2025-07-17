package workflow

import (
	"fmt"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/pkg/errors"
)

type StatusPhaseMeta struct {
	MetadatasBase   *MetadatasBase
	MachineStatuses []*StatusMachineMeta
}

type StatusMachineMeta struct {
	BaseMeta          *executioner.BaseMeta
	Reachable         bool
	SSHConnectable    bool
	Bootstrapped      bool
	CurrentGeneration string
	LastDeployTime    string
}

// executeStatusPhase runs status checks in parallel across all machines
// and must complete fully before proceeding to next phase
func (w *WorkflowExecutor) ExecuteStatusPhase() error {
	if w.meta.StatusPhaseMeta == nil {
		w.meta.StatusPhaseMeta = &StatusPhaseMeta{}
	}
	spm := w.meta.StatusPhaseMeta

	if spm.MachineStatuses == nil {
		spm.MachineStatuses = make([]*StatusMachineMeta, 0)
	}

	if w.cfg.Global.Verbose {
		fmt.Println("Executing status phase across all flake configurations")
	}

	if spm.MetadatasBase == nil {
		spm.MetadatasBase = &MetadatasBase{}
	}

	err := w.forEachFlakeConfiguration(spm.MetadatasBase, func(wp *WorkflowExecutorForConfigurationAndMachine, bm *executioner.BaseMeta, configuration *config.Configuration) error {
		if w.cfg.Global.Verbose {
			fmt.Println("Executing status phase across all machines in " + bm.FlakeName + " " + bm.ConfigurationName)
		}

		return w.forEachConfigurationMachine(configuration, spm.MetadatasBase, bm, func(wp *WorkflowExecutorForConfigurationAndMachine, bm *executioner.BaseMeta, machine *config.Machine) error {
			sm := &StatusMachineMeta{BaseMeta: bm}
			spm.MachineStatuses = append(spm.MachineStatuses, sm)

			if w.cfg.Global.Verbose {
				fmt.Println("Executing status phase on machine " + bm.MachineName.String())
			}

			return wp.executeStatusPhaseMachineStreams(sm, machine, w.hook.OnUpdateHook())
		})
	})

	return err
}

func (w *WorkflowExecutorForConfigurationAndMachine) executeStatusPhaseMachineStreams(sm *StatusMachineMeta, machine *config.Machine, onUpdateHook func()) error {
	exc := executioner.New(w.ctx, sm.BaseMeta, onUpdateHook, w.cfg, machine)

	// TCP check
	err := exc.PingStream(
		func(bm *executioner.BaseMeta, err error) error {
			return fmt.Errorf("machine unreachable: %w", err)
		},
		func(bm *executioner.BaseMeta) {
			sm.Reachable = true
		})
	if err != nil {
		return err
	}

	// SSH connect
	err = exc.Exec(
		func(bm *executioner.BaseMeta, err error) error {
			return errors.Wrapf(err, "ssh test failed: %s", bm.CommandOutputs[len(bm.CommandOutputs)-1].StdCombined.String())
		},
		func(bm *executioner.BaseMeta) {
			sm.SSHConnectable = true
		}, "sh", "-c", "exit 0")
	if err != nil {
		return err
	}

	// Run bootstrap detection
	err = exc.Exec(
		nil,
		func(bm *executioner.BaseMeta) {
			sm.Bootstrapped = true
		}, "sh", "-c", "test -e /run/current-system")
	if err != nil {
		return nil // just not bootstrapped, not really an error
	}

	// Get current generation
	err = exc.Exec(
		nil,
		func(bm *executioner.BaseMeta) {
			sm.CurrentGeneration = strings.TrimSpace(bm.CommandOutputs[len(bm.CommandOutputs)-1].Stdout.String())
		}, "sh", "-c", "nixos-rebuild list-generations | tail -1 | awk '{print $1}'")
	if err != nil {
		return err
	}

	// Get last deploy time
	err = exc.Exec(
		nil,
		func(bm *executioner.BaseMeta) {
			sm.LastDeployTime = strings.TrimSpace(bm.CommandOutputs[len(bm.CommandOutputs)-1].Stdout.String())
		}, "sh", "-c", "stat -c %Y /run/current-system 2>/dev/null | xargs -I {} date -d @{} '+%Y-%m-%d %H:%M:%S' || echo 'unknown'")
	if err != nil {
		return err
	}

	return nil
}
