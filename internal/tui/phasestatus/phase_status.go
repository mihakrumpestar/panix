package phasestatus

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/flow"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

type Selected struct {
	Phase string `json:"phase,omitempty"`
	Index int    `json:"index"`
}

type PhaseStatus struct {
	fleet       *fleet.Fleet
	colorScheme *colorscheme.ColorScheme
	pf          *flow.PhaseFlow
	phases      []phase.Phase
	Selected    Selected
}

func NewPhaseStatus(fleet *fleet.Fleet, colorScheme *colorscheme.ColorScheme, workflowPhases []phase.Phase) *PhaseStatus {
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
		SelectionBg:     colorToStyleColor(cs.Table.SelectionHighlightBackground.GetBackground()),
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

	return &PhaseStatus{
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

func colorToStyleColor(c color.Color) style.Color {
	r, g, b, _ := c.RGBA()
	return style.Color(fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8))
}

func (p *PhaseStatus) View(width int) string {
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

	header := p.colorScheme.Header.Title.Render("=== Phase Status ===")

	return header + "\n\n" + result + "\n\n"
}

func (p *PhaseStatus) syncSelection() {
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

func (p *PhaseStatus) Reset() {
	p.pf.Reset()
	p.syncSelection()
}

func (p *PhaseStatus) HandleMouseClick(msg render.MouseClickMsg) bool {
	if p.pf.HandleMouseClick(msg) {
		p.syncSelection()
		return true
	}

	return false
}

func (p *PhaseStatus) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if p.pf.HandleNavigation(key, hasActiveInnerViewport) {
		p.syncSelection()
		return true
	}

	return false
}
