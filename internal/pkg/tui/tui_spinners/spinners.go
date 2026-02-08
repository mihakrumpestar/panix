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
	spinners, _ := omap.New[config_attributes.Xpath, *Spinner]()

	return &Spinners{
		spinners: spinners,
	}
}

func (s *Spinners) GetOrCreateSpinner(xpath config_attributes.Xpath) *spinner.Model {
	if spnr, ok := s.spinners.Get(xpath); ok {
		return spnr.model
	}

	spnrRaw := spinner.New(spinner.WithSpinner(spinner.Dot))
	spnr := &Spinner{
		model: &spnrRaw,
	}

	s.spinners.Set(xpath, spnr)

	return spnr.model
}

func (s *Spinners) RemoveIfExists(xpath config_attributes.Xpath) {
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
	cmds := make([]tea.Cmd, 0, s.spinners.Len())

	for _, spnr := range s.spinners.Records() {
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
