package atomictimeandstate

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
)

type AtomicTimeAndState struct {
	*atomicpointer.AtomicPointer[TimeAndState]
}

func New() *AtomicTimeAndState {
	return &AtomicTimeAndState{atomicpointer.New(&TimeAndState{live: true})}
}

func (tas *AtomicTimeAndState) StartTimer() {
	startTime := tas.Load().StartTime
	if !startTime.IsZero() {
		return
	}

	timeNow := time.Now()
	tas.Update(func(tas *TimeAndState) {
		tas.StartTime = timeNow
	})
}

func (tas *AtomicTimeAndState) EndTimerWithError(err error) {
	if tas.Load().IsFinished() {
		return
	}

	timeNow := time.Now()
	tas.Update(func(tas *TimeAndState) {
		tas.DurationCache = timeNow.Sub(tas.StartTime)
		tas.EndTime = timeNow
		tas.EndError = errorjson.New(err)
	})
}

func (t *AtomicTimeAndState) UnmarshalJSON(data []byte) error {
	if t.AtomicPointer == nil {
		t.AtomicPointer = atomicpointer.New(&TimeAndState{})
	}

	return t.AtomicPointer.UnmarshalJSON(data)
}
