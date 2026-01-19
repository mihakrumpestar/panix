package logs

import "github.com/mihakrumpestar/panix/internal/workflow/phases"

type StatsState string

const (
	Running StatsState = "running"
	Failed  StatsState = "failed"
	Done    StatsState = "done" // Only for the "done" phase
)

type StatisticsPerPhase struct {
	stats map[phases.Phase]map[StatsState]uint
}

func NewStatisticsPerPhase() *StatisticsPerPhase {
	return &StatisticsPerPhase{
		stats: map[phases.Phase]map[StatsState]uint{},
	}
}

func (spp *StatisticsPerPhase) Increment(phase phases.Phase, state StatsState) {
	phaseStats, ok := spp.stats[phase]
	if !ok {
		phaseStats = map[StatsState]uint{state: 1}
		spp.stats[phase] = phaseStats
		return
	}

	phaseStateStats, ok := phaseStats[state]
	if !ok {
		phaseStats[state] = 1
		return
	}

	phaseStateStats++
}

type StatsPack struct {
	Running uint
	Failed  uint
	Done    uint
}

func (spp *StatisticsPerPhase) GetPack(phase phases.Phase) *StatsPack {
	return &StatsPack{
		spp.Get(phase, Running),
		spp.Get(phase, Failed),
		spp.Get(phase, Done),
	}
}

func (spp *StatisticsPerPhase) Get(phase phases.Phase, state StatsState) uint {
	phaseStats, ok := spp.stats[phase]
	if !ok {
		return 0
	}

	phaseStateStats, ok := phaseStats[state]
	if !ok {
		return 0
	}

	return phaseStateStats
}
