package spinners

import (
	"fmt"
	"strings"
	"time"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinner"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/zeroterm"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

type tickMsg struct{}

type Spinners struct {
	entries *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *spinner.Spinner]
	cs      colorscheme.ColorSchemeSpinner
	viewed  bool
	ticking bool
}

func New(cs colorscheme.ColorSchemeSpinner) *Spinners {
	return &Spinners{
		entries: atomicorderedmap.New[xpath.Xpath, *spinner.Spinner](),
		cs:      cs,
	}
}

func (s *Spinners) View(xpath xpath.Xpath) string {
	s.viewed = true

	sp, ok := s.entries.Get(xpath)
	if ok {
		return sp.View()
	}

	sp = spinner.New(s.cs.Frames, s.cs.Interval)
	s.entries.Set(xpath, sp)

	return sp.View()
}

func (s *Spinners) ProcessPendingTicks() zeroterm.Cmd {
	if s == nil || s.ticking || !s.viewed {
		return nil
	}

	s.viewed = false
	s.ticking = true

	return s.nextTick()
}

func (s *Spinners) Update(msg zeroterm.Msg) zeroterm.Cmd {
	if _, ok := msg.(tickMsg); !ok {
		return nil
	}

	for _, pair := range s.entries.Pairs() {
		pair.Value.Update()
	}

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
}

func (s *Spinners) Debug() string {
	var str strings.Builder
	fmt.Fprintf(&str, "\nSpinners: %d (ticking: %v)\n", s.entries.Len(), s.ticking)

	return str.String()
}

func (s *Spinners) nextTick() zeroterm.Cmd {
	return zeroterm.TickCmd(s.cs.Interval, func(time.Time) zeroterm.Msg { return tickMsg{} })
}
