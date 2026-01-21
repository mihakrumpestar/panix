package logs_stats

import (
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type StatsState string

const (
	Running StatsState = "running"
	Failed  StatsState = "failed"
	Done    StatsState = "done" // Only for the "done" phase
)

type StatisticsPerPhase struct {
	stats map[phases.Phase]map[StatsState][]config_attributes.Xpath
}

func NewStatisticsPerPhase() *StatisticsPerPhase {
	return &StatisticsPerPhase{
		stats: map[phases.Phase]map[StatsState][]config_attributes.Xpath{},
	}
}

func (spp *StatisticsPerPhase) Add(phase phases.Phase, state StatsState, Xpath config_attributes.Xpath) {
	phaseStats, ok := spp.stats[phase]
	if !ok {
		phaseStats = map[StatsState][]config_attributes.Xpath{}
		spp.stats[phase] = phaseStats
	}

	phaseStateStats, ok := phaseStats[state]
	if !ok {
		phaseStateStats = []config_attributes.Xpath{}
		phaseStats[state] = phaseStateStats
	}

	phaseStats[state] = append(phaseStateStats, Xpath)
}

func (spp *StatisticsPerPhase) Get(phase phases.Phase, state StatsState) []config_attributes.Xpath {
	phaseStats, ok := spp.stats[phase]
	if !ok {
		return []config_attributes.Xpath{}
	}

	phaseStateStats, ok := phaseStats[state]
	if !ok {
		return []config_attributes.Xpath{}
	}

	return phaseStateStats
}

type StatsPack struct {
	Running []config_attributes.Xpath
	Failed  []config_attributes.Xpath
	Done    []config_attributes.Xpath
}

func (spp *StatisticsPerPhase) GetPack(phase phases.Phase) *StatsPack {
	return &StatsPack{
		spp.Get(phase, Running),
		spp.Get(phase, Failed),
		spp.Get(phase, Done),
	}
}
