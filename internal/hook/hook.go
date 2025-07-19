package hook

type Hook struct {
	channel   chan uint64
	iteration uint64
}

func NewHook() *Hook {
	return &Hook{
		make(chan uint64),
		0,
	}
}

func (h *Hook) GetChannel() <-chan uint64 {
	return h.channel
}

func (h *Hook) Close() {
	close(h.channel)
}

func (h *Hook) OnUpdateHook() {
	h.iteration++
	select {
	case h.channel <- h.iteration:
		// Successfully sent update
	default:
		// Channel is full or no receiver, skip without blocking
	}
}
