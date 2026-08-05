package fleet

import (
	"context"
	"iter"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/pkg/stringbyte"
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

	// cachedTAS persists TimeAndState across frames so that stateVersion
	// accumulates correctly. Cleared on ResetState (workflow restart).
	cachedTAS []*atomictimeandstate.TimeAndState

	// cachedStats is reused across frames via Reset() to avoid allocating
	// a new StatisticsPerPhase map structure every frame.
	cachedStats *stats.StatisticsPerPhase
}

type MachineInfo struct {
	Xpath       xpath.Xpath
	MetaInspect machine.MetaInspect
	State       machine.State

	// Pre-converted []byte cells for the stats table renderer.
	// StringByte provides zero-copy Bytes() access.
	MachineName       stringbyte.StringByte
	FlakeName         stringbyte.StringByte
	OutputType        stringbyte.StringByte
	OutputName        stringbyte.StringByte
	Architecture      stringbyte.StringByte
	Date              stringbyte.StringByte
	OSVersion         stringbyte.StringByte
	Kernel            stringbyte.StringByte
	StatusMsgBytes    stringbyte.StringByte
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

// PostUnmarshalInit recomputes derived state that is not serialized (json:"-").
// Must be called after JSON deserialization (e.g. snapshot loading).
func (f *Fleet) PostUnmarshalInit() {
	f.Flakes.ForEach(func(_ string, flakeV *flake.Flake) bool {
		if flakeV == nil {
			return true
		}

		if flakeV.Logs != nil {
			flakeV.Logs.PostUnmarshalInit()
		}

		flakeV.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]) bool {
			if attrMap == nil {
				return true
			}

			attrMap.ForEach(func(_ string, cfg *installable.Installable) bool {
				if cfg == nil {
					return true
				}

				if cfg.Logs != nil {
					cfg.Logs.PostUnmarshalInit()
				}

				cfg.Machines.ForEach(func(_ string, mach *machine.Machine) bool {
					postUnmarshalMachine(mach)

					return true
				})

				return true
			})

			return true
		})

		return true
	})
}

func postUnmarshalMachine(mach *machine.Machine) {
	if mach == nil {
		return
	}

	mach.PostUnmarshalInit(mach.Name.String(), nil)

	if mach.Logs == nil {
		return
	}

	mach.Logs.PhaseLogs.ForEach(func(_ phase.Phase, phaseLog *phaselogs.PhaseLog) bool {
		if phaseLog == nil || phaseLog.CommandLogs == nil {
			return true
		}

		phaseLog.CommandLogs.ForEach(func(_ int, cmd *command.CommandLog) bool {
			if cmd != nil {
				cmd.PostUnmarshalInit()
			}

			return true
		})

		return true
	})
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
	machineCount := f.MachineCount()

	// Grow slices if needed, reuse existing capacity.
	if cap(f.CacheFlattenedLogs) >= machineCount {
		f.CacheFlattenedLogs = f.CacheFlattenedLogs[:machineCount]
	} else {
		f.CacheFlattenedLogs = make([]*logs.Logs, machineCount)
	}

	if cap(f.cachedTAS) >= machineCount {
		f.cachedTAS = f.cachedTAS[:machineCount]
	} else {
		newSlice := make([]*atomictimeandstate.TimeAndState, machineCount)
		copy(newSlice, f.cachedTAS)
		f.cachedTAS = newSlice
	}

	for machineIdx, treeLeaf := range f.AllMachines() {
		if f.CacheFlattenedLogs[machineIdx] == nil {
			f.CacheFlattenedLogs[machineIdx] = logs.New()
		}

		if f.cachedTAS[machineIdx] == nil {
			f.cachedTAS[machineIdx] = &atomictimeandstate.TimeAndState{}
		}

		// Merge phase logs in-place into the cached Logs entry.
		// dst.TAS acts as a temp snapshot, synced into the persisted cachedTAS below.
		logs.MergePhaseLogsInto(f.CacheFlattenedLogs[machineIdx], workflowPhases,
			treeLeaf.Machine.Logs.PhaseLogs,
			treeLeaf.Installable.Logs.PhaseLogs,
			treeLeaf.Flake.Logs.PhaseLogs,
		)

		// Sync aggregated state into persisted TAS; bumps stateVersion on transitions.
		f.cachedTAS[machineIdx].SyncFrom(f.CacheFlattenedLogs[machineIdx].TAS)
	}
}

func (f *Fleet) RecalculateDurationAndError() {
	idx := 0

	var largestFlakeDuration time.Duration

	f.Flakes.ForEach(func(_ string, flakeV *flake.Flake) bool {
		var largestOutputDuration time.Duration

		flakeAnyRunning := false

		flakeV.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]) bool {
			if attrMap == nil {
				return true
			}

			var machineCount int

			largestOutputDuration, flakeAnyRunning, machineCount = f.recalcInstallableDurations(
				attrMap, idx, largestOutputDuration, flakeAnyRunning,
			)

			idx += machineCount

			return true
		})

		propagateDurationAndState(flakeV.Logs, largestOutputDuration, flakeAnyRunning)

		if largestOutputDuration > largestFlakeDuration {
			largestFlakeDuration = largestOutputDuration
		}

		return true
	})

	f.Logs.TAS.SetDuration(largestFlakeDuration)
}

// propagateDurationAndState sets duration on the TAS and transitions state
// based on whether any child entity is still running.
func propagateDurationAndState(target *logs.Logs, duration time.Duration, anyRunning bool) {
	target.TAS.SetDuration(duration)

	if anyRunning {
		if !target.TAS.HasStarted() {
			target.TAS.SetStarted(time.Now())
		} else if target.TAS.IsFinished() {
			target.TAS.MarkRunning()
		}
	} else if !target.TAS.IsFinished() {
		target.TAS.MarkFinished()
	}
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
				machineState.StatusMsg = stringbyte.StringByte(lastCommandLog.StatusIfRunning)

				return
			}

			if endErr != nil && errors.Is(errors.Cause(endErr.Err()), context.Canceled) {
				machineState.Status = stats.Running
				machineState.StatusMsg = stringbyte.StringByte(lastCommandLog.StatusIfRunning)
				machineState.Error = nil

				return
			}

			if endErr != nil {
				machineState.Status = stats.Failed
				machineState.StatusMsg = stringbyte.StringByte(lastCommandLog.StatusIfFailed)
				machineState.Error = endErr

				return
			}

			machineState.Error = nil

			if workflowPhases[len(workflowPhases)-1] == machineState.Phase {
				machineState.Status = stats.Done
				machineState.StatusMsg = "done"
			} else {
				machineState.Status = stats.Done
				machineState.StatusMsg = stringbyte.StringByte(machineState.Phase.String() + " done")
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
	f.cachedStats = nil

	// Clear persisted TAS so stateVersion resets for the new workflow.
	for i := range f.cachedTAS {
		f.cachedTAS[i] = nil
	}

	f.Flakes.ForEach(func(_ string, flakeV *flake.Flake) bool {
		flakeV.Logs.Clear()

		flakeV.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]) bool {
			if attrMap == nil {
				return true
			}

			attrMap.ForEach(func(_ string, installable *installable.Installable) bool {
				installable.Logs.Clear()

				installable.Machines.ForEach(func(_ string, machineV *machine.Machine) bool {
					machineV.Logs.Clear()
					machineV.MetaInspect.Clear()

					return true
				})

				return true
			})

			return true
		})

		return true
	})
}

// Helpers

type FleetLeaf struct {
	Flake       *flake.Flake
	Installable *installable.Installable
	Machine     *machine.Machine
}

func (f *Fleet) AllMachines() iter.Seq2[int, *FleetLeaf] {
	return func(yield func(int, *FleetLeaf) bool) {
		idx := 0

		f.Flakes.ForEach(func(_ string, flakeV *flake.Flake) bool {
			return flakeV.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]) bool {
				if attrMap == nil {
					return true
				}

				return attrMap.ForEach(func(_ string, installable *installable.Installable) bool {
					return installable.Machines.ForEach(func(_ string, machineV *machine.Machine) bool {
						if !yield(idx, &FleetLeaf{flakeV, installable, machineV}) {
							return false
						}

						idx++

						return true
					})
				})
			})
		})
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
	if f.cachedStats == nil {
		f.cachedStats = stats.New(workflowPhases)
	} else {
		f.cachedStats.Reset()
	}

	for _, fleetLeaf := range f.AllMachines() {
		mach := fleetLeaf.Machine
		machState := mach.State.Load()

		// Skip machines that haven't started any phase yet; their Phase and
		// Status are empty strings. Adding them via DeepSet would insert a ""
		// key into the ordered map, which shifts spp.Last() away from the real
		// last workflow phase and breaks the PhaseFlow "DONE" column.
		if machState.Phase == "" || machState.Status == "" {
			continue
		}

		f.cachedStats.DeepSet(machState.Phase, machState.Status, mach.Xpath)
	}

	f.CacheStatisticsPerPhase = f.cachedStats

	return f.cachedStats
}

func (f *Fleet) RefreshCaches() {
	idx := 0

	for _, treeLeaf := range f.AllMachines() {
		flake := treeLeaf.Flake
		out := treeLeaf.Installable
		mach := treeLeaf.Machine

		meta := mach.MetaInspect.Load()
		state := mach.State.Load()

		// Grow slice if needed.
		if idx >= len(f.CacheMachineInfos) {
			f.CacheMachineInfos = append(f.CacheMachineInfos, MachineInfo{})
		}

		mInfo := &f.CacheMachineInfos[idx]

		mInfo.Xpath = mach.Xpath
		mInfo.MetaInspect = *meta
		mInfo.State = *state

		// Direct StringByte copies: zero conversion, zero allocation.
		mInfo.MachineName = mach.Name
		mInfo.FlakeName = flake.Name
		mInfo.OutputType = stringbyte.StringByte(out.Type)
		mInfo.OutputName = stringbyte.StringByte(out.Name)
		mInfo.Architecture = meta.Architecture
		mInfo.Date = meta.Date
		mInfo.OSVersion = meta.OSVersion
		mInfo.Kernel = meta.Kernel
		mInfo.StatusMsgBytes = state.StatusMsg

		idx++
	}

	// Truncate if machine count decreased.
	f.CacheMachineInfos = f.CacheMachineInfos[:idx]
}

// recalcInstallableDurations processes all installables in an inner map, computing
// the largest machine duration and whether any machine is still running.
// Returns the updated largestOutputDuration, flakeAnyRunning flag, and the
// total number of machines processed (for advancing the outer index).
func (f *Fleet) recalcInstallableDurations(
	attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable],
	idx int,
	largestOutputDuration time.Duration,
	flakeAnyRunning bool,
) (time.Duration, bool, int) {
	machineCount := 0

	attrMap.ForEach(func(_ string, installable *installable.Installable) bool {
		if installable == nil {
			return true
		}

		var largestMachineDuration time.Duration

		cfgAnyRunning := false

		installable.Machines.ForEach(func(_ string, machineV *machine.Machine) bool {
			machineV.Logs.TAS.SyncFrom(f.cachedTAS[idx])

			machineDur := machineV.Logs.TAS.DurationCache
			if machineDur > largestMachineDuration {
				largestMachineDuration = machineDur
			}

			if !machineV.Logs.TAS.IsFinished() {
				cfgAnyRunning = true
			}

			idx++
			machineCount++

			return true
		})

		propagateDurationAndState(installable.Logs, largestMachineDuration, cfgAnyRunning)

		if largestMachineDuration > largestOutputDuration {
			largestOutputDuration = largestMachineDuration
		}

		if !installable.Logs.TAS.IsFinished() {
			flakeAnyRunning = true
		}

		return true
	})

	return largestOutputDuration, flakeAnyRunning, machineCount
}
