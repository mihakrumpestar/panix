package config

import (
	"sync"
	"time"
)

type TimeAndState struct {
	mutex     sync.Mutex
	startTime time.Time
	endTime   time.Time
	started   bool
	finished  bool
	error     error
}

func NewTimeAndState() *TimeAndState {
	return &TimeAndState{}
}

func (tas *TimeAndState) StartTimer() {
	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	tas.startTime = time.Now()
	tas.started = true
}

func (tas *TimeAndState) EndTimer() {
	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	tas.endTime = time.Now()
	tas.finished = true
}

func (tas *TimeAndState) EndTimerWithError(err error) {
	tas.EndTimer()

	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	tas.error = err
}

func (tas *TimeAndState) GetTimeAndState() TimeAndStateCopy {
	tas.mutex.Lock()
	defer tas.mutex.Unlock()

	return TimeAndStateCopy{
		StartTime: tas.startTime,
		EndTime:   tas.endTime,
		Started:   tas.started,
		Finished:  tas.finished,
		Error:     tas.error,
	}
}

type TimeAndStateCopy struct {
	StartTime time.Time
	EndTime   time.Time
	Started   bool
	Finished  bool
	Error     error
}
