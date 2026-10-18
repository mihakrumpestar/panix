package spinner

import (
	"time"
)

// Interval bounds: a tick faster than the minimum is a hot loop, and one slower
// than the maximum is indistinguishable from a stalled spinner. Config validation
// and the runtime scheduler share these bounds.
const (
	MinInterval = 10 * time.Millisecond
	MaxInterval = 60 * time.Second
)

// ClampInterval returns interval clamped into [MinInterval, MaxInterval].
func ClampInterval(interval time.Duration) time.Duration {
	if interval < MinInterval {
		return MinInterval
	}

	if interval > MaxInterval {
		return MaxInterval
	}

	return interval
}

// Spinner is an animated spinner component with internal frame state.
type Spinner struct {
	frames   [][]byte
	interval time.Duration
	frame    int
}

// New creates a new spinner with the given animation frames and tick interval.
func New(frames [][]byte, interval time.Duration) *Spinner {
	spinner := &Spinner{}
	spinner.ApplyTheme(frames, interval)

	return spinner
}

// ApplyTheme re-runs the constructor styling step: replaces the animation
// frames and tick interval. The current frame position is kept; Render wraps
// it into the new frame range.
func (s *Spinner) ApplyTheme(frames [][]byte, interval time.Duration) {
	s.frames = frames
	s.interval = ClampInterval(interval)
}

// Update advances the spinner to the next frame.
func (s *Spinner) Update() {
	if len(s.frames) == 0 {
		return
	}

	s.frame = (s.frame + 1) % len(s.frames)
}

// Render returns the current frame string.
func (s *Spinner) Render() []byte {
	if len(s.frames) == 0 {
		return nil
	}

	return s.frames[s.frame%len(s.frames)]
}

// Interval returns the time between frames.
func (s *Spinner) Interval() time.Duration {
	return s.interval
}
