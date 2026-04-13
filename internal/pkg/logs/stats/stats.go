package stats

import (
	"github.com/mihakrumpestar/panix/internal/pkg/orderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type StatsState string

const (
	Running StatsState = "running"
	Failed  StatsState = "failed"
	Done    StatsState = "done" // Only for the "done" phase
)

type StatisticsPerPhase struct {
	*orderedmap.OrderedMap[phases.Phase, StatsPack]
}

type StatsPack map[StatsState][]xpath.Xpath

func New() *StatisticsPerPhase {
	return &StatisticsPerPhase{
		orderedmap.New[phases.Phase, StatsPack](),
	}
}

func (spp *StatisticsPerPhase) DeepSet(phase phases.Phase, statsState StatsState, xpathI xpath.Xpath) {
	statsPack, ok := spp.OrderedMap.Get(phase)
	if !ok {
		statsPack = StatsPack{}
		spp.OrderedMap.Set(phase, statsPack)
	}

	xpaths, ok := statsPack[statsState]
	if !ok {
		xpaths = make([]xpath.Xpath, 0)
		statsPack[statsState] = xpaths
	}

	xpaths = append(xpaths, xpathI)
}
