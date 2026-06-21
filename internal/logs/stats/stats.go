package stats

import (
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
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
		atomicorderedmap.NewWithCap[phase.Phase, StatsPack](len(workflowPhases)),
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

	return errors.Wrap(spp.AtomicOrderedMap.UnmarshalJSON(data), "unmarshal statistics per phase")
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
