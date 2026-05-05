package spinner

import "time"

// Spinner is an animated spinner component with internal frame state.
type Spinner struct {
	frames   []string
	interval time.Duration
	frame    int
}

// New creates a new spinner with the given animation frames and tick interval.
func New(frames []string, interval time.Duration) *Spinner {
	return &Spinner{frames: frames, interval: interval}
}

// Update advances the spinner to the next frame.
func (s *Spinner) Update() {
	if len(s.frames) == 0 {
		return
	}

	s.frame = (s.frame + 1) % len(s.frames)
}

// View returns the current frame string.
func (s *Spinner) View() string {
	if len(s.frames) == 0 {
		return ""
	}

	return s.frames[s.frame%len(s.frames)]
}

// Interval returns the time between frames.
func (s *Spinner) Interval() time.Duration {
	return s.interval
}
