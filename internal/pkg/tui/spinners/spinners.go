package spinners

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
)

type tickMsg struct{}

type Spinners struct {
	entries *omap.Omap[xpath.Xpath, *entry]
	ticking bool
}

type entry struct {
	lastUsed time.Time
	model    spinner.Model
}

func NewSpinners() (*Spinners, error) {
	entries, err := omap.New[xpath.Xpath, *entry]()
	if err != nil {
		return nil, err
	}

	return &Spinners{entries: entries}, nil
}

func (s *Spinners) View(xpath xpath.Xpath) string {
	if e, ok := s.entries.Get(xpath); ok {
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
	if _, ok := msg.(tickMsg); !ok {
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
