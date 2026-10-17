// Spinner frame patterns from charm.land/bubbles/v2/spinner.
// See pkg/tui/LICENSE.charmbracelet.

package spinners

import (
	"fmt"
	"time"

	"github.com/mihakrumpestar/panix/pkg/buffer"

	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/tui/spinner"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

type tickMsg struct{}

type Spinners struct {
	entries  *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *spinner.Spinner]
	frames   [][]byte
	interval time.Duration

	viewed     bool
	ticking    bool
	generation uint64
}

func New(frames [][]byte, interval time.Duration) *Spinners {
	spinners := &Spinners{
		entries: atomicorderedmap.New[xpath.Xpath, *spinner.Spinner](),
	}
	spinners.ApplyTheme(frames, interval)

	return spinners
}

// ApplyTheme re-runs the constructor styling step: replaces the spinner
// frames and tick interval, restyling every live spinner in place.
func (s *Spinners) ApplyTheme(frames [][]byte, interval time.Duration) {
	interval = spinner.ClampInterval(interval)
	s.frames = frames
	s.interval = interval

	s.entries.ForEach(func(_ xpath.Xpath, sp *spinner.Spinner) bool {
		sp.ApplyTheme(frames, interval)

		return true
	})
}

// Render returns the spinner frame for the given xpath as a byte slice.
func (s *Spinners) Render(xpathVal xpath.Xpath) []byte {
	s.viewed = true

	spinnerI, ok := s.entries.Get(xpathVal)
	if !ok {
		spinnerI = spinner.New(s.frames, s.interval)
		s.entries.Set(xpathVal, spinnerI)
	}

	return spinnerI.Render()
}

func (s *Spinners) ProcessPendingTicks() zeroterm.Cmd {
	if s == nil || s.ticking || !s.viewed {
		return nil
	}

	s.viewed = false
	s.ticking = true

	return s.nextTick()
}

// Generation returns a counter that increments on each spinner tick.
// Used to throttle display updates to the spinner frame rate (10fps).
func (s *Spinners) Generation() uint64 {
	return s.generation
}

func (s *Spinners) Update(msg zeroterm.Msg) zeroterm.Cmd {
	if _, ok := msg.(tickMsg); !ok {
		return nil
	}

	s.generation++

	s.entries.ForEach(func(_ xpath.Xpath, sp *spinner.Spinner) bool {
		sp.Update()

		return true
	})

	if s.viewed {
		s.viewed = false

		return s.nextTick()
	}

	s.ticking = false

	return nil
}

func (s *Spinners) Reset() {
	s.entries.Clear()
	s.viewed = false
	s.ticking = false
	s.generation = 0
}

func (s *Spinners) Debug(buf *buffer.LinesBuf) {
	buf.EmptyLine()
	buf.WriteString(fmt.Sprintf("Spinners: %d (ticking: %v)", s.entries.Len(), s.ticking))
}

func (s *Spinners) nextTick() zeroterm.Cmd {
	return zeroterm.TickCmd(spinner.ClampInterval(s.interval), func(time.Time) zeroterm.Msg { return tickMsg{} })
}
