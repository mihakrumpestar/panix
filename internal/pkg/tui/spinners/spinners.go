package spinners

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

type tickMsg struct{}

type Spinners struct {
	entries *atomicorderedmap.AtomicOrderedMap[xpath.Xpath, *entry]
	ticking bool
}

type entry struct {
	lastUsed time.Time
	model    spinner.Model
}

func NewSpinners() (*Spinners, error) {
	return &Spinners{entries: atomicorderedmap.New[xpath.Xpath, *entry]()}, nil
}

func (s *Spinners) View(xpath xpath.Xpath) string {
	e, ok := s.entries.Get(xpath)
	if ok {
		e.lastUsed = time.Now()

		return e.model.View()
	}

	model := spinner.New(spinner.WithSpinner(spinner.Dot))
	s.entries.Set(xpath, &entry{
		lastUsed: time.Now(),
		model:    model,
	})

	return model.View()
}

func (s *Spinners) ProcessPendingTicks() tea.Cmd {
	if s == nil || s.ticking {
		return nil
	}

	if s.entries.Len() == 0 {
		return nil
	}

	s.ticking = true

	return s.nextTick()
}

func (s *Spinners) Update(msg tea.Msg) tea.Cmd {
	_, ok := msg.(tickMsg)
	if !ok {
		return nil
	}

	now := time.Now()
	for _, pair := range s.entries.Pairs() {
		if now.Sub(pair.Value.lastUsed).Seconds() > 1 {
			s.entries.Del(pair.Key)
		}
	}

	if s.entries.Len() == 0 {
		s.ticking = false

		return nil
	}

	for _, pair := range s.entries.Pairs() {
		newModel, _ := pair.Value.model.Update(spinner.TickMsg{})
		pair.Value.model = newModel
	}

	return s.nextTick()
}

func (s *Spinners) Debug() string {
	var str strings.Builder
	fmt.Fprintf(&str, "\nSpinners: %d (ticking: %v)\n", s.entries.Len(), s.ticking)

	return str.String()
}

// helpers

func (s *Spinners) nextTick() tea.Cmd {
	return tea.Tick(spinner.Dot.FPS, func(time.Time) tea.Msg { return tickMsg{} })
}
