package config

import "time"

type TimeAndState struct {
	startTime time.Time
	endTime   time.Time
	started   bool
	finished  bool
	error     error
}

func (tas *TimeAndState) StartTimer() {
	tas.startTime = time.Now()
	tas.started = true
}

func (tas *TimeAndState) EndTimer() {
	tas.endTime = time.Now()
	tas.finished = true
}

func (tas *TimeAndState) EndTimerWithError(err error) {
	tas.EndTimer()
	tas.error = err
}

func (tas *TimeAndState) GetTimeAndState() TimeAndStateOutput {
	if tas == nil {
		return TimeAndStateOutput{}
	}

	return TimeAndStateOutput{
		tas.startTime,
		tas.endTime,
		tas.started,
		tas.finished,
		tas.error,
	}
}

type TimeAndStateOutput struct {
	StartTime time.Time
	EndTime   time.Time
	Started   bool
	Finished  bool
	Error     error
}
