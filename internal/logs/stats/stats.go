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

	statsPack[statsState] = append(statsPack[statsState], xpathI)
}

// Reset clears all xpath slices in every StatsPack while preserving the map
// structure and backing capacity. After Reset, all slices have len==0 but
// retain their cap, so subsequent DeepSet appends are zero-allocation.
// Must be called from the same goroutine that calls DeepSet (single-writer).
func (spp *StatisticsPerPhase) Reset() {
	spp.ForEach(func(_ phase.Phase, statsPack StatsPack) bool {
		for state := range statsPack {
			statsPack[state] = statsPack[state][:0]
		}

		return true
	})
}
