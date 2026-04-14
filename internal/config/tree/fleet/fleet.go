package fleet

import (
	"iter"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/tui/phasestatus"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type Fleet struct {
	attributes.Attributes `yaml:",inline"`

	Flakes atomicorderedmap.AtomicOrderedMap[string, *flake.Flake] `yaml:"flakes,required" json:"flakes"`

	// Internal
	Logs *logs.Logs `yaml:"-" json:"logs,omitempty"`

	// For TUI representation
	StatsTable  *statstable.StatsTable   `yaml:"-" json:"stats_table"`
	PhaseStatus *phasestatus.PhaseStatus `yaml:"-" json:"phase_status"`
}

func (r *Fleet) Init(f flags.Flags) error {
	err := r.Attributes.Init("", attributes.New(f), false)
	if err != nil {
		return errors.Wrap(err, "failed to initialize fleet attributes")
	}

	r.Logs = logs.New()

	r.StatsTable = statstable.NewStatsTable()
	r.PhaseStatus = phasestatus.NewPhaseStatus()

	return nil
}

func (f *Fleet) Recalculate(workflowPhases []phases.Phase) {
	f.RecalculateFlattenedLogs(workflowPhases)
	f.RecalculateDurationAndError()
	f.RecalculateMachinesState(workflowPhases)

	f.RefreshStatsTable()
	f.RecalculatePhaseStatus(workflowPhases)
}

// RecalculateCachesOnly rebuilds derived caches (flattened logs, stats table, phase status)
// without overwriting machine state. Use after snapshot deserialization where State and durations are already correct.
func (f *Fleet) RecalculateCachesOnly(workflowPhases []phases.Phase) {
	f.RecalculateFlattenedLogs(workflowPhases)

	f.RefreshStatsTable()
	f.RecalculatePhaseStatus(workflowPhases)
}

func (f *Fleet) RecalculateFlattenedLogs(workflowPhases []phases.Phase) {
	flattenedLogs := make([]*logs.Logs, 0)

	for _, treeLeaf := range f.AllMachines() {
		mergedLogs := logs.MergePhaseLogs(workflowPhases,
			treeLeaf.Machine.Logs.PhaseLogs,
			treeLeaf.Configuration.Logs.PhaseLogs,
			treeLeaf.Flake.Logs.PhaseLogs,
		)

		flattenedLogs = append(flattenedLogs, mergedLogs)
	}

	f.StatsTable.CacheFlattenedLogs = flattenedLogs
}

func (f *Fleet) RecalculateDurationAndError() {
	i := 0

	var largestFlakeDuration time.Duration

	for _, flake := range f.Flakes.Pairs() {
		var largestConfigurationDuration time.Duration

		for _, configuration := range flake.Value.Configurations.Pairs() {
			var largestMachineDuration time.Duration

			for _, machine := range configuration.Value.Machines.Pairs() {
				dae := f.StatsTable.CacheFlattenedLogs[i].DurationAndErrorCache

				machine.Value.Logs.DurationAndErrorCache = dae

				if dae.Duration > largestMachineDuration {
					largestMachineDuration = dae.Duration
				}

				i++
			}

			configuration.Value.Logs.DurationAndErrorCache.Duration = largestMachineDuration

			if largestMachineDuration > largestConfigurationDuration {
				largestConfigurationDuration = largestMachineDuration
			}
		}

		flake.Value.Logs.DurationAndErrorCache.Duration = largestConfigurationDuration

		if largestConfigurationDuration > largestFlakeDuration {
			largestFlakeDuration = largestConfigurationDuration
		}
	}

	f.Logs.DurationAndErrorCache.Duration = largestFlakeDuration
}

func (f *Fleet) RecalculateMachinesState(workflowPhases []phases.Phase) {
	for i, fleetLeaf := range f.AllMachines() {
		machineState := fleetLeaf.Machine.State

		machineLastPhaseLog, ok := f.GetMachineLastPhaseLog(i)
		if !ok {
			continue
		}

		machineState.Phase = machineLastPhaseLog.Key

		lastCommandLog, ok := machineLastPhaseLog.Value.CommandLogs.Last()
		if !ok {
			continue
		}

		tas := machineLastPhaseLog.Value.TimeAndState.Load()
		endErr := tas.EndError
		machineState.Error = endErr // Error (or lack thereof) has to be always propagated (e.g. retry)

		if !tas.IsFinished() {
			machineState.Status = stats.Running
			machineState.StatusMsg = lastCommandLog.StatusIfRunning
			continue
		}

		if endErr != nil {
			machineState.Status = stats.Failed
			machineState.StatusMsg = lastCommandLog.StatusIfFailed
			continue
		}

		if workflowPhases[len(workflowPhases)-1] == machineState.Phase {
			machineState.Status = stats.Done
			machineState.StatusMsg = "done"
		} else {
			machineState.Status = stats.Done
			machineState.StatusMsg = string(machineState.Phase) + " done"
		}
	}
}

func (f *Fleet) GetMachineLastPhaseLog(machineInOrder int) (atomicorderedmap.Pair[phases.Phase, *phase.PhaseLog], bool) {
	return f.StatsTable.CacheFlattenedLogs[machineInOrder].PhaseLogs.Last()
}

func (f *Fleet) ResetState() {
	f.Logs.Clear()
	f.StatsTable = statstable.NewStatsTable()
	f.PhaseStatus = phasestatus.NewPhaseStatus()

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
				machineV.State.ActiveSSH = machine.SSHTypeRegular
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
		i := 0
		for _, flake := range f.Flakes.Pairs() {
			for _, configuration := range flake.Value.Configurations.Pairs() {
				for _, machine := range configuration.Value.Machines.Pairs() {
					if !yield(i, &FleetLeaf{flake.Value, configuration.Value, machine.Value}) {
						return
					}

					i++
				}
			}
		}
	}
}

func (f *Fleet) RecalculatePhaseStatus(workflowPhases []phases.Phase) *stats.StatisticsPerPhase {
	statisticsPerPhase := stats.New(workflowPhases)

	for _, treeLeaf := range f.AllMachines() {
		ms := treeLeaf.Machine

		statisticsPerPhase.DeepSet(ms.State.Phase, ms.State.Status, ms.Xpath)
	}

	f.PhaseStatus.CacheStatisticsPerPhase = statisticsPerPhase

	return statisticsPerPhase
}

func (f *Fleet) RefreshStatsTable() {
	machineInfos := make([]statstable.MachineInfo, 0)

	for _, treeLeaf := range f.AllMachines() {
		m := treeLeaf.Machine

		mInfo := statstable.MachineInfo{
			Xpath:       m.Xpath,
			MetaInspect: *m.MetaInspect.Load(),
			State:       *m.State,
		}

		machineInfos = append(machineInfos, mInfo)
	}

	f.StatsTable.CacheMachineInfos = machineInfos
}
