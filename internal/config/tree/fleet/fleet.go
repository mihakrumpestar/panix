package fleet

import (
	"context"
	"iter"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
)

type Fleet struct {
	attributes.Attributes `yaml:",inline"`

	Nix    nix.NixConfig                                            `yaml:"nix" json:"nix" desc:"Nix build and copy configuration"`
	Flakes *atomicorderedmap.AtomicOrderedMap[string, *flake.Flake] `yaml:"flakes,required" json:"flakes" desc:"Flakes in the fleet"`

	// Internal
	Logs *logs.Logs `yaml:"-" json:"logs,omitempty"`

	// Caches for TUI representation
	CacheMachineInfos       []MachineInfo             `yaml:"-" json:"-"`
	CacheFlattenedLogs      []*logs.Logs              `yaml:"-" json:"-"`
	CacheStatisticsPerPhase *stats.StatisticsPerPhase `yaml:"-" json:"-"`
}

type MachineInfo struct {
	Xpath       xpath.Xpath
	MetaInspect machine.MetaInspect
	State       machine.State
}

func (f *Fleet) Init() error {
	err := f.Attributes.Init("", attributes.New())
	if err != nil {
		return errors.Wrap(err, "failed to initialize fleet attributes")
	}

	err = f.Nix.Init(nil)
	if err != nil {
		return errors.Wrap(err, "failed to initialize fleet nix config")
	}

	f.Logs = logs.New()

	return nil
}

func (f *Fleet) Recalculate(workflowPhases []phase.Phase) {
	f.RecalculateFlattenedLogs(workflowPhases)
	f.RecalculateDurationAndError()
	f.RecalculateMachinesState(workflowPhases)

	f.RefreshCaches()
	f.RecalculatePhaseStatus(workflowPhases)
}

func (f *Fleet) RecalculateCachesOnly(workflowPhases []phase.Phase) {
	f.RecalculateFlattenedLogs(workflowPhases)

	f.RefreshCaches()
	f.RecalculatePhaseStatus(workflowPhases)
}

func (f *Fleet) RecalculateFlattenedLogs(workflowPhases []phase.Phase) {
	flattenedLogs := make([]*logs.Logs, 0)

	for _, treeLeaf := range f.AllMachines() {
		mergedLogs := logs.MergePhaseLogs(workflowPhases,
			treeLeaf.Machine.Logs.PhaseLogs,
			treeLeaf.Configuration.Logs.PhaseLogs,
			treeLeaf.Flake.Logs.PhaseLogs,
		)

		flattenedLogs = append(flattenedLogs, mergedLogs)
	}

	f.CacheFlattenedLogs = flattenedLogs
}

func (f *Fleet) RecalculateDurationAndError() {
	idx := 0

	var largestFlakeDuration time.Duration

	for _, flake := range f.Flakes.Pairs() {
		var largestConfigurationDuration time.Duration

		for _, configuration := range flake.Value.Configurations.Pairs() {
			var largestMachineDuration time.Duration

			for _, machine := range configuration.Value.Machines.Pairs() {
				dae := f.CacheFlattenedLogs[idx].DurationAndErrorCache
				machine.Value.Logs.SetDurationAndError(dae)

				if dae.Duration > largestMachineDuration {
					largestMachineDuration = dae.Duration
				}

				idx++
			}

			configuration.Value.Logs.SetDuration(largestMachineDuration)

			if largestMachineDuration > largestConfigurationDuration {
				largestConfigurationDuration = largestMachineDuration
			}
		}

		flake.Value.Logs.SetDuration(largestConfigurationDuration)

		if largestConfigurationDuration > largestFlakeDuration {
			largestFlakeDuration = largestConfigurationDuration
		}
	}

	f.Logs.SetDuration(largestFlakeDuration)
}

func (f *Fleet) RecalculateMachinesState(workflowPhases []phase.Phase) {
	for i, fleetLeaf := range f.AllMachines() {
		machineLastPhaseLog, ok := f.GetMachineLastPhaseLog(i)
		if !ok {
			continue
		}

		lastCommandLog, ok := machineLastPhaseLog.Value.CommandLogs.Last()
		if !ok {
			continue
		}

		tas := machineLastPhaseLog.Value.TimeAndState.Load()
		endErr := tas.EndError

		fleetLeaf.Machine.State.Update(func(machineState *machine.State) {
			machineState.Phase = machineLastPhaseLog.Key

			if !tas.IsFinished() {
				machineState.Status = stats.Running
				machineState.StatusMsg = lastCommandLog.StatusIfRunning

				return
			}

			if endErr != nil && errors.Is(errors.Cause(endErr.Err()), context.Canceled) {
				machineState.Status = stats.Running
				machineState.StatusMsg = lastCommandLog.StatusIfRunning
				machineState.Error = nil

				return
			}

			if endErr != nil {
				machineState.Status = stats.Failed
				machineState.StatusMsg = lastCommandLog.StatusIfFailed
				machineState.Error = endErr

				return
			}

			machineState.Error = nil

			if workflowPhases[len(workflowPhases)-1] == machineState.Phase {
				machineState.Status = stats.Done
				machineState.StatusMsg = "done"
			} else {
				machineState.Status = stats.Done
				machineState.StatusMsg = machineState.Phase.String() + " done"
			}
		})
	}
}

func (f *Fleet) GetMachineLastPhaseLog(machineInOrder int) (atomicorderedmap.Pair[phase.Phase, *phaselogs.PhaseLog], bool) {
	return f.CacheFlattenedLogs[machineInOrder].PhaseLogs.Last()
}

func (f *Fleet) ResetState() {
	f.Logs.Clear()
	f.CacheMachineInfos = nil
	f.CacheFlattenedLogs = nil
	f.CacheStatisticsPerPhase = nil

	for _, flakeP := range f.Flakes.Pairs() {
		flakeV := flakeP.Value

		flakeV.Logs.Clear()

		for _, configurationP := range flakeV.Configurations.Pairs() {
			configurationV := configurationP.Value

			configurationV.Logs.Clear()

			for _, machineP := range configurationV.Machines.Pairs() {
				machineV := machineP.Value

				machineV.Logs.Clear()
				machineV.MetaInspect.Clear()
			}
		}
	}
}

// Helpers

type FleetLeaf struct {
	Flake         *flake.Flake
	Configuration *configuration.Configuration
	Machine       *machine.Machine
}

func (f *Fleet) AllMachines() iter.Seq2[int, *FleetLeaf] {
	return func(yield func(int, *FleetLeaf) bool) {
		idx := 0

		for _, flake := range f.Flakes.Pairs() {
			for _, configuration := range flake.Value.Configurations.Pairs() {
				for _, machine := range configuration.Value.Machines.Pairs() {
					if !yield(idx, &FleetLeaf{flake.Value, configuration.Value, machine.Value}) {
						return
					}

					idx++
				}
			}
		}
	}
}

func (f *Fleet) MachineCount() int {
	count := 0

	for range f.AllMachines() {
		count++
	}

	return count
}

func (f *Fleet) RecalculatePhaseStatus(workflowPhases []phase.Phase) *stats.StatisticsPerPhase {
	statisticsPerPhase := stats.New(workflowPhases)

	for _, treeLeaf := range f.AllMachines() {
		ms := treeLeaf.Machine
		msState := ms.State.Load()

		statisticsPerPhase.DeepSet(msState.Phase, msState.Status, ms.Xpath)
	}

	f.CacheStatisticsPerPhase = statisticsPerPhase

	return statisticsPerPhase
}

func (f *Fleet) RefreshCaches() {
	machineInfos := make([]MachineInfo, 0, f.MachineCount())

	for _, treeLeaf := range f.AllMachines() {
		m := treeLeaf.Machine

		mInfo := MachineInfo{
			Xpath:       m.Xpath,
			MetaInspect: *m.MetaInspect.Load(),
			State:       *m.State.Load(),
		}

		machineInfos = append(machineInfos, mInfo)
	}

	f.CacheMachineInfos = machineInfos
}
