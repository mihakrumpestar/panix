package spinners

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/kirill-scherba/omap"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
)

const maxSpinners = 5

type Spinners struct {
	spinners   *omap.Omap[int, *Spinner]
	pendingCmd tea.Cmd
}

type Spinner struct {
	lastUsed time.Time
	model    *spinner.Model
}

func NewSpinners() (*Spinners, error) {
	spinners, err := omap.New[int, *Spinner]()
	if err != nil {
		return nil, err
	}

	return &Spinners{
		spinners: spinners,
	}, nil
}

func (s *Spinners) GetOrCreateSpinner(xpath attributes.Xpath) *spinner.Model {
	h := fnv.New32a()
	h.Write([]byte(xpath.String()))
	hashKey := int(h.Sum32() % maxSpinners)

	spnr, ok := s.spinners.Get(hashKey)
	if ok {
		spnr.lastUsed = time.Now()

		return spnr.model
	}

	spnrRaw := spinner.New(spinner.WithSpinner(spinner.Dot))
	spnr = &Spinner{
		lastUsed: time.Now(),
		model:    &spnrRaw,
	}

	s.spinners.Set(hashKey, spnr)
	s.pendingCmd = tea.Batch(s.pendingCmd, spnr.model.Tick)

	return spnr.model
}

func (s *Spinners) ProcessPendingTicks() tea.Cmd {
	if s == nil {
		return nil
	}

	// Delete spinners that have not been rendered in View for more than x time
	now := time.Now()
	for _, spinner := range s.spinners.Pairs() {
		if now.Sub(spinner.Value.lastUsed).Seconds() > 1 {
			s.spinners.Del(spinner.Key)
		}
	}

	cmd := s.pendingCmd
	s.pendingCmd = nil

	return cmd
}

func (s *Spinners) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(spinner.TickMsg); !ok {
		return nil
	}

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

	for key := range s.spinners.Records() {
		fmt.Fprintf(&str, "  key=%d\n", key)
	}

	return str.String()
}
