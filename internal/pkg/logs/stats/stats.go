package stats

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type StatsState string

const (
	Running StatsState = "running"
	Failed  StatsState = "failed"
	Done    StatsState = "done" // Only for the "done" phase
)

type StatisticsPerPhase struct {
	stats map[phases.Phase]map[StatsState][]attributes.Xpath
}

func NewStatisticsPerPhase() *StatisticsPerPhase {
	return &StatisticsPerPhase{
		stats: make(map[phases.Phase]map[StatsState][]attributes.Xpath),
	}
}

func (spp *StatisticsPerPhase) Add(phase phases.Phase, state StatsState, xpath attributes.Xpath) {
	phaseStats, ok := spp.stats[phase]
	if !ok {
		phaseStats = make(map[StatsState][]attributes.Xpath)
		spp.stats[phase] = phaseStats
	}

	phaseStats[state] = append(phaseStats[state], xpath)
}

func (spp *StatisticsPerPhase) Get(phase phases.Phase, state StatsState) []attributes.Xpath {
	phaseStats, ok := spp.stats[phase]
	if !ok {
		return nil
	}

	result, ok := phaseStats[state]
	if !ok {
		return nil
	}

	return result
}

func (spp *StatisticsPerPhase) Clear() {
	spp.stats = make(map[phases.Phase]map[StatsState][]attributes.Xpath)
}

type StatsPack struct {
	Running []attributes.Xpath
	Failed  []attributes.Xpath
	Done    []attributes.Xpath
}

func (spp *StatisticsPerPhase) GetPack(phase phases.Phase) *StatsPack {
	if spp == nil {
		return nil
	}

	return &StatsPack{
		Running: spp.Get(phase, Running),
		Failed:  spp.Get(phase, Failed),
		Done:    spp.Get(phase, Done),
	}
}
