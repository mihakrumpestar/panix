package hook

import "sync/atomic"

type Hook struct {
	channel   chan uint64
	iteration atomic.Uint64
}

func NewHook() *Hook {
	return &Hook{
		make(chan uint64),
		atomic.Uint64{},
	}
}

func (h *Hook) WaitForUpdate() <-chan uint64 {
	return h.channel
}

func (h *Hook) Close() {
	close(h.channel)
}

func (h *Hook) OnUpdateHook() {
	select {
	case h.channel <- h.iteration.Add(1):
		// Successfully sent update
	default:
		// Channel is full or no receiver, skip without blocking
	}
}
