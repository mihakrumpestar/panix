package phaseflow

import (
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/tui/flow"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

type Selected struct {
	Phase string `json:"phase,omitempty"`
	Index int    `json:"index"`
}

type PhaseFlow struct {
	fleet       *fleet.Fleet
	colorScheme *colorscheme.ColorScheme
	pf          *flow.PhaseFlow
	phases      []phase.Phase
	Selected    Selected
}

func New(fleet *fleet.Fleet, colorScheme *colorscheme.ColorScheme, workflowPhases []phase.Phase) *PhaseFlow {
	cs := colorScheme

	pfStyles := flow.Styles{
		GradientRunning: colorfulPair(cs.PhaseStatus.Running),
		GradientFailed:  colorfulPair(cs.PhaseStatus.Failed),
		GradientDone:    colorfulPair(cs.PhaseStatus.Done),
		GradientDefault: colorfulPair(cs.PhaseStatus.Default),
		Pill:            cs.PhaseStatus.Pill,
		StatusRunning:   cs.Status.Running,
		StatusFailed:    cs.Status.Failed,
		StatusDone:      cs.Status.OK,
		StatusSeparator: cs.Table.Border,
		Arrow:           cs.Table.Border,
		PhaseArrow:      cs.Chars.PhaseArrow,
		SelectionBg:     cs.Table.SelectionHighlightBackground.GetBackground(),
	}

	names := make([]string, 0, len(workflowPhases)+1)
	for _, p := range workflowPhases {
		names = append(names, strings.ToUpper(p.String()))
	}

	names = append(names, "DONE")

	pf := flow.New().
		Phases(names...).
		Styles(pfStyles).
		SetZonePrefix("phase-status")

	return &PhaseFlow{
		fleet:       fleet,
		colorScheme: colorScheme,
		pf:          pf,
		phases:      workflowPhases,
		Selected:    Selected{Index: -1},
	}
}

func colorfulPair(cp colorscheme.ColorPair) flow.GradientPair {
	return flow.GradientPair{Dark: cp[0], Light: cp[1]}
}

func (p *PhaseFlow) View(width int) string {
	spp := p.fleet.CacheStatisticsPerPhase
	if spp == nil {
		return ""
	}

	pairs := spp.Pairs()
	data := make([]flow.PhaseData, 0, len(pairs)+1)

	for _, pair := range pairs {
		pd := flow.PhaseData{}
		if pair.Value != nil {
			pd.Running = len(pair.Value[stats.Running])
			pd.Failed = len(pair.Value[stats.Failed])
		}

		data = append(data, pd)
	}

	lastPair, _ := spp.Last()
	doneData := flow.PhaseData{
		Done: len(lastPair.Value[stats.Done]),
	}

	data = append(data, doneData)

	p.pf.Width(width).SetData(data)

	result := p.pf.String()
	p.syncSelection()

	header := p.colorScheme.Header.Title.Render("=== Phase Flow ===")

	return header + "\n\n" + result + "\n\n"
}

func (p *PhaseFlow) Reset() {
	p.pf.Reset()
	p.syncSelection()
}

func (p *PhaseFlow) HandleMouseClick(msg zeroterm.MouseClickMsg) bool {
	if p.pf.HandleMouseClick(msg) {
		p.syncSelection()

		return true
	}

	return false
}

func (p *PhaseFlow) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if p.pf.HandleNavigation(key, hasActiveInnerViewport) {
		p.syncSelection()

		return true
	}

	return false
}

// Helpers

func (p *PhaseFlow) syncSelection() {
	idx := p.pf.SelectedIndex()
	p.Selected.Index = idx

	if idx >= 0 {
		if idx < len(p.phases) {
			p.Selected.Phase = p.phases[idx].String()
		} else {
			p.Selected.Phase = "done"
		}
	} else {
		p.Selected.Phase = ""
	}
}
