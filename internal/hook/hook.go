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

func (h *Hook) Done() <-chan uint64 {
	return h.channel
}

func (h *Hook) Close() {
	close(h.channel)
}

func (h *Hook) OnUpdateHook() func() {
	return func() {
		h.iteration++
		h.channel <- h.iteration
	}
}
