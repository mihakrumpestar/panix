package tui_spinners

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
)

type Spinners struct {
	spinners   *omap.Omap[config_attributes.Xpath, *Spinner]
	pendingCmd tea.Cmd
}

type Spinner struct {
	model *spinner.Model
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
	s.pendingCmd = tea.Batch(s.pendingCmd, spnr.model.Tick)

	return spnr.model
}

func (s *Spinners) RemoveIfExists(xpath config_attributes.Xpath) {
	s.spinners.Del(xpath)
}

func (s *Spinners) ProcessPendingTicks() tea.Cmd {
	if s == nil {
		return nil
	}

	cmd := s.pendingCmd
	s.pendingCmd = nil
	return cmd
}

func (s *Spinners) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	for _, spnr := range s.spinners.Records() {
		spinnerModel, spinnerCmd := spnr.model.Update(msg)
		if spinnerCmd != nil {
			cmd = tea.Batch(cmd, spinnerCmd)
			*spnr.model = spinnerModel
		}
	}

	return cmd
}

func (s *Spinners) Debug() string {
	var str strings.Builder
	fmt.Fprintf(&str, "\nSpinners: %d\n", s.spinners.Len())

	for pathx := range s.spinners.Records() {
		fmt.Fprintf(&str, "  '%s'\n", pathx)
	}

	return str.String()
}
