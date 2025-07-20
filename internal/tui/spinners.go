package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/elliotchance/orderedmap/v3"
)

type Spinners struct {
	spinners *orderedmap.OrderedMap[string, *Spinner]
}

type Spinner struct {
	used         bool
	initTickSend bool
	model        *spinner.Model
}

func NewSpinners() *Spinners {
	return &Spinners{
		spinners: orderedmap.NewOrderedMap[string, *Spinner](),
	}
}

func (s *Spinners) Spinner(xpath string) *spinner.Model {
	// Existing spinner
	spnr, ok := s.spinners.Get(xpath)
	if ok {
		spnr.used = true
		return spnr.model
	}

	// New spinner
	spnrRaw := spinner.New(spinner.WithSpinner(spinner.Dot))
	spnr = &Spinner{
		used:  true,
		model: &spnrRaw,
	}

	s.spinners.Set(xpath, spnr)

	return spnr.model
}

func (s *Spinners) TickAndClean(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	for xpath, spnr := range s.spinners.AllFromFront() {
		if !spnr.used {
			s.spinners.Delete(xpath)
			continue
		}

		if !spnr.initTickSend {
			spnr.initTickSend = true
			cmds = append(cmds, spnr.model.Tick)
		}
	}

	return tea.Batch(cmds...)
}

func (s *Spinners) UpdateAndClean(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	for xpath, spnr := range s.spinners.AllFromFront() {
		if !spnr.used {
			s.spinners.Delete(xpath)
			continue
		}

		spinnerModel, cmd := spnr.model.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
			*spnr.model = spinnerModel
		}
	}

	return tea.Batch(cmds...)
}

func (s *Spinners) BeforeViewConstructionHook() {
	for _, spnr := range s.spinners.AllFromFront() {
		spnr.used = false
	}
}

func (s *Spinners) Debug() string {
	str := fmt.Sprintf("\nSpinners: %d\n", s.spinners.Len())

	for pathx, spnr := range s.spinners.AllFromFront() {
		str += fmt.Sprintf("  %v: '%s'\n", spnr.used, pathx)
	}

	return str
}
