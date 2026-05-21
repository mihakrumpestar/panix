package phaseflow

import (
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/buffer"
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
	content     *buffer.LinesBuf
}

func New(fleet *fleet.Fleet, scheme *colorscheme.ColorScheme, workflowPhases []phase.Phase) *PhaseFlow {
	pfStyles := flow.Styles{
		GradientRunning: colorfulPair(scheme.PhaseStatus.Running),
		GradientFailed:  colorfulPair(scheme.PhaseStatus.Failed),
		GradientDone:    colorfulPair(scheme.PhaseStatus.Done),
		GradientDefault: colorfulPair(scheme.PhaseStatus.Default),
		Pill:            scheme.PhaseStatus.Pill,
		Status: flow.StatusStyles{
			Running: scheme.Status.Running,
			Failed:  scheme.Status.Failed,
			Done:    scheme.Status.OK,
		},
		StatusSeparator: scheme.Table.Border,
		Arrow:           scheme.Table.Border,
		PhaseArrow:      scheme.Chars.PhaseArrow,
		SelectionBg:     scheme.Table.SelectionHighlightBackground.GetBackground(),
	}

	names := make([]string, 0, len(workflowPhases)+1)
	for _, p := range workflowPhases {
		names = append(names, strings.ToUpper(p.String()))
	}

	names = append(names, "DONE")

	phaseFlow := flow.New().
		Phases(names...).
		Styles(pfStyles).
		SetZonePrefix("phase-status")

	return &PhaseFlow{
		fleet:       fleet,
		colorScheme: scheme,
		pf:          phaseFlow,
		phases:      workflowPhases,
		Selected:    Selected{Index: -1},
		content:     buffer.NewLinesBuf(),
	}
}

func colorfulPair(cp colorscheme.ColorPair) flow.GradientPair {
	return flow.GradientPair{Dark: cp[0], Light: cp[1]}
}

// Render renders the phase flow and returns the output buffer.
func (p *PhaseFlow) Render(width int) *buffer.LinesBuf {
	spp := p.fleet.CacheStatisticsPerPhase
	if spp == nil {
		p.content.Reset()

		return p.content
	}

	pairs := spp.Pairs()
	data := make([]flow.PhaseData, 0, len(pairs)+1)

	for _, pair := range pairs {
		phaseData := flow.PhaseData{}
		if pair.Value != nil {
			phaseData.Running = len(pair.Value[stats.Running])
			phaseData.Failed = len(pair.Value[stats.Failed])
		}

		data = append(data, phaseData)
	}

	lastPair, _ := spp.Last()
	doneData := flow.PhaseData{
		Done: len(lastPair.Value[stats.Done]),
	}

	data = append(data, doneData)

	p.pf.Width(width).SetData(data)

	p.content.Reset()

	phaseFlowHeader := [][]byte{
		p.colorScheme.Header.Title.RenderLine([]byte("=== Phase Flow ===")),
		[]byte{},
	}

	p.content.WriteLines(phaseFlowHeader)
	p.content.AppendFrom(p.pf.Render())
	p.content.EmptyLine()

	return p.content
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
