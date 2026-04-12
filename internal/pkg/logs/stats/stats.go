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
	*orderedmap.OrderedMap[phases.Phase, *StatsPack]
}

type StatsPack struct {
	Running []xpath.Xpath
	Failed  []xpath.Xpath
	Done    []xpath.Xpath
}

func New() *StatisticsPerPhase {
	return &StatisticsPerPhase{
		orderedmap.New[phases.Phase, *StatsPack](),
	}
}
