package retry

import "sync"

type Retry struct {
	mutex sync.Mutex
	retry chan struct{}
}

func NewTaskRetry() *Retry {
	return &Retry{
		retry: make(chan struct{}),
	}
}

// Pauses execution
func (tr *Retry) Wait() {
	tr.mutex.Lock()
	retry := tr.retry
	tr.mutex.Unlock()

	<-retry
}

func (tr *Retry) Trigger() {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	close(tr.retry)
	tr.retry = make(chan struct{})
}
