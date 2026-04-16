package stats

import (
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

type StatsState string

const (
	Running StatsState = "running"
	Failed  StatsState = "failed"
	Done    StatsState = "done" // Only for the "done" phase
)

type StatisticsPerPhase struct {
	*atomicorderedmap.AtomicOrderedMap[phase.Phase, StatsPack]
}

type StatsPack map[StatsState][]xpath.Xpath

func New(workflowPhases []phase.Phase) *StatisticsPerPhase {
	spp := &StatisticsPerPhase{
		atomicorderedmap.New[phase.Phase, StatsPack](),
	}

	for _, phase := range workflowPhases {
		spp.AtomicOrderedMap.Set(phase, StatsPack{})
	}

	return spp
}

func (spp *StatisticsPerPhase) UnmarshalJSON(data []byte) error {
	if spp.AtomicOrderedMap == nil {
		spp.AtomicOrderedMap = atomicorderedmap.New[phase.Phase, StatsPack]()
	}

	return spp.AtomicOrderedMap.UnmarshalJSON(data)
}

func (spp *StatisticsPerPhase) DeepSet(phase phase.Phase, statsState StatsState, xpathI xpath.Xpath) {
	statsPack, ok := spp.AtomicOrderedMap.Get(phase)
	if !ok {
		statsPack = StatsPack{}
		spp.AtomicOrderedMap.Set(phase, statsPack)
	}

	xpaths, ok := statsPack[statsState]
	if !ok {
		xpaths = make([]xpath.Xpath, 0)
	}

	statsPack[statsState] = append(xpaths, xpathI)
}
