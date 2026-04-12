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
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/orderedmap"
	"github.com/mihakrumpestar/panix/internal/tui"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

type Fleet struct {
	attributes.Attributes `yaml:",inline"`

	Flakes orderedmap.OrderedMap[string, *flake.Flake] `yaml:"flakes,required" json:"flakes"`

	// Internal
	Logs *logs.Logs `yaml:"-" json:"logs,omitempty"`

	// For TUI representation
	StatsTable  *StatsTable      `yaml:"-" json:"stats_table"`
	PhaseStatus *tui.PhaseStatus `yaml:"-" json:"phase_status"`
}

type StatsTable struct {
	MachineInfos    []MachineInfo `json:"-"`
	SelectedMachine int           `json:"selected_machine"`

	CacheFlattenedLogs []*logs.Logs `json:"-"`
	CacheHash          uint64       `json:"-"`
	CacheTableContent  string       `json:"-"`
}

func (r *Fleet) Init(f *flags.Flags) error {
	err := r.Attributes.Init("fleet", &attributes.Attributes{Flags: f}, false)
	if err != nil {
		return errors.Wrap(err, "failed to initialize fleet attributes")
	}

	r.Logs.PhaseLogs = phase.NewPhaseLogs()

	return nil
}

func (f *Fleet) Recalculate(workflowPhases []phases.Phase) {
	f.RecalculateFlattenedLogs(workflowPhases)
	f.RecalculateDurationAndError()
	f.RecalculateMachinesState(workflowPhases)
	f.RefreshStatsTable()
}

func (f *Fleet) RecalculateFlattenedLogs(workflowPhases []phases.Phase) {
	flattenedLogs := make([]*logs.Logs, 0)

	for i, treeLeaf := range f.AllMachines() {
		flattenedLogs[i] = logs.MergePhaseLogs(workflowPhases,
			treeLeaf.Machine.Logs.PhaseLogs,
			treeLeaf.Configuration.Logs.PhaseLogs,
			treeLeaf.Flake.Logs.PhaseLogs,
		)
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
			largestConfigurationDuration = largestConfigurationDuration
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

		tas := machineLastPhaseLog.Value.TimeAndState.Load()
		if !tas.IsFinished() {
			machineState.Status = stats.Running
			continue
		}

		endErr := tas.EndError
		if endErr != nil {
			machineState.Status = stats.Failed
			machineState.Error = endErr
			continue
		}

		// Last phase completed without error, mark machine as done
		if workflowPhases[len(workflowPhases)-1] == machineState.Phase {
			machineState.Status = stats.Done
		}
	}
}

func (f *Fleet) GetMachineLastPhaseLog(machineInOrder int) (orderedmap.Pair[phases.Phase, *phase.PhaseLog], bool) {
	return f.StatsTable.CacheFlattenedLogs[machineInOrder].PhaseLogs.Last()
}

func (f *Fleet) ResetState() {
	f.Logs.Clear()
	f.StatsTable = nil
	f.PhaseStatus = nil

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

func (f *Fleet) AllMachines() iter.Seq2[int, FleetLeaf] {
	return func(yield func(int, FleetLeaf) bool) {
		i := 0
		for _, flake := range f.Flakes.Pairs() {
			for _, configuration := range flake.Value.Configurations.Pairs() {
				for _, machine := range configuration.Value.Machines.Pairs() {
					if !yield(i, FleetLeaf{flake.Value, configuration.Value, machine.Value}) {
						return
					}

					i++
				}
			}
		}
	}
}

func (f *Fleet) RecalculatePhaseStatus() *stats.StatisticsPerPhase {
	statisticsPerPhase := stats.NewStatisticsPerPhase()

	for _, treeLeaf := range f.AllMachines() {
		ms := treeLeaf.Machine

		statisticsPerPhase.Add(ms.State.Phase, ms.State.Status, ms.Xpath)
	}

	f.PhaseStatus.StatisticsPerPhase = statisticsPerPhase

	return statisticsPerPhase
}

type MachineInfo struct {
	*machine.MetaInspect
	*machine.State
}

func (f *Fleet) RefreshStatsTable() {
	machineInfos := make([]MachineInfo, 0)

	for _, treeLeaf := range f.AllMachines() {
		m := treeLeaf.Machine

		machineInfos = append(machineInfos, MachineInfo{m.MetaInspect.Load(), m.State})
	}

	f.StatsTable.MachineInfos = machineInfos
}
