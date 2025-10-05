package time_and_state

import (
	"sync"
	"time"
)

type TimeAndState struct {
	mutex    sync.Mutex
	internal *TimeAndStateInternal
}

type TimeAndStateInternal struct {
	StartTime time.Time
	EndTime   time.Time
	Started   bool
	Finished  bool
	Error     error
}

func NewTimeAndState() *TimeAndState {
	return &TimeAndState{internal: &TimeAndStateInternal{}}
}

func (tas *TimeAndState) StartTimer() {
	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	tas.internal.StartTime = time.Now()
	tas.internal.Started = true
}

func (tas *TimeAndState) EndTimer() {
	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	tas.internal.EndTime = time.Now()
	tas.internal.Finished = true
}

func (tas *TimeAndState) EndTimerWithError(err error) {
	tas.EndTimer()

	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	tas.internal.Error = err
}

// Returns a copy
func (tas *TimeAndState) GetTimeAndState() TimeAndStateInternal {
	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	copy := *tas.internal
	return copy
}
