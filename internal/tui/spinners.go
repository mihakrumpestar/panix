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
	initTickSend bool
	model        *spinner.Model
}

func NewSpinners() *Spinners {
	return &Spinners{
		spinners: orderedmap.NewOrderedMap[string, *Spinner](),
	}
}

func (s *Spinners) GetOrCreateSpinner(xpath string) *spinner.Model {
	// Existing spinner
	spnr, ok := s.spinners.Get(xpath)
	if ok {
		return spnr.model
	}

	// New spinner
	spnrRaw := spinner.New(spinner.WithSpinner(spinner.Dot))
	spnr = &Spinner{
		model: &spnrRaw,
	}

	s.spinners.Set(xpath, spnr)

	return spnr.model
}

func (s *Spinners) RemoveIfExistsSpinner(xpath string) {
	s.spinners.Delete(xpath)
}

func (s *Spinners) SendInitTickIfNotAlready() tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	for _, spnr := range s.spinners.AllFromFront() {
		if !spnr.initTickSend {
			cmds = append(cmds, spnr.model.Tick)
			spnr.initTickSend = true
		}
	}

	return tea.Batch(cmds...)
}

func (s *Spinners) Update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	for _, spnr := range s.spinners.AllFromFront() {
		// msg only works on spinner it was ment to, so we don't have to filter or anything
		spinnerModel, cmd := spnr.model.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
			*spnr.model = spinnerModel
		}
	}

	return tea.Batch(cmds...)
}

func (s *Spinners) Debug() string {
	str := fmt.Sprintf("\nSpinners: %d\n", s.spinners.Len())

	for pathx := range s.spinners.Keys() {
		str += fmt.Sprintf("  '%s'\n", pathx)
	}

	return str
}
