package tui_spinners

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
)

type Spinners struct {
	spinners *omap.Omap[config_attributes.Xpath, *Spinner]
}

type Spinner struct {
	initTickSend bool
	model        *spinner.Model
}

func NewSpinners() *Spinners {
	spinners, err := omap.New[config_attributes.Xpath, *Spinner]()
	if err != nil {
		panic(err)
	}

	return &Spinners{
		spinners: spinners,
	}
}

func (s *Spinners) GetOrCreateSpinner(xpath config_attributes.Xpath) *spinner.Model {
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

func (s *Spinners) RemoveIfExistsSpinner(xpath config_attributes.Xpath) {
	s.spinners.Del(xpath)
}

func (s *Spinners) SendInitTickIfNotAlready() tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	for _, spnr := range s.spinners.Records() {
		if !spnr.initTickSend {
			cmds = append(cmds, spnr.model.Tick)
			spnr.initTickSend = true
		}
	}

	return tea.Batch(cmds...)
}

func (s *Spinners) Update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	for _, spnr := range s.spinners.Records() {
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

	for pathx := range s.spinners.Records() {
		str += fmt.Sprintf("  '%s'\n", pathx)
	}

	return str
}
