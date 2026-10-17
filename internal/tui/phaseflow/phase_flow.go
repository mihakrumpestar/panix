package phaseflow

import (
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/theme"
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
	fleet    *fleet.Fleet
	scheme   *theme.Theme
	pf       *flow.PhaseFlow
	phases   []phase.Phase
	Selected Selected
	content  *buffer.LinesBuf
	data     []flow.PhaseData
}

var phaseFlowHeader = []byte("=== Phase Flow ===")

func New(fleet *fleet.Fleet, scheme *theme.Theme, workflowPhases []phase.Phase) *PhaseFlow {
	names := make([]string, 0, len(workflowPhases)+1)
	for _, p := range workflowPhases {
		names = append(names, strings.ToUpper(p.String()))
	}

	names = append(names, "DONE")

	phaseFlow := flow.New().
		Phases(names...).
		SetZonePrefix("phase-status")

	phaseFlowState := &PhaseFlow{
		fleet:    fleet,
		pf:       phaseFlow,
		phases:   workflowPhases,
		Selected: Selected{Index: -1},
		content:  buffer.NewLinesBuf(),
	}
	phaseFlowState.ApplyTheme(scheme)

	return phaseFlowState
}

// ApplyTheme re-applies the flow styles derived from the given theme.
func (p *PhaseFlow) ApplyTheme(scheme *theme.Theme) {
	p.scheme = scheme
	p.pf.ApplyTheme(flowStyles(scheme))
}

// flowStyles derives the flow styles from the theme.
func flowStyles(scheme *theme.Theme) flow.Styles {
	return flow.Styles{
		GradientRunning: flow.GradientPairFromColors(scheme.PhaseStatus.Running[0], scheme.PhaseStatus.Running[1]),
		GradientFailed:  flow.GradientPairFromColors(scheme.PhaseStatus.Failed[0], scheme.PhaseStatus.Failed[1]),
		GradientDone:    flow.GradientPairFromColors(scheme.PhaseStatus.Done[0], scheme.PhaseStatus.Done[1]),
		GradientDefault: flow.GradientPairFromColors(scheme.PhaseStatus.Default[0], scheme.PhaseStatus.Default[1]),
		Pill:            scheme.PhaseStatus.Pill.Style,
		Status: flow.StatusStyles{
			Running: scheme.Status.Running.Style,
			Failed:  scheme.Status.Failed.Style,
			Done:    scheme.Status.OK.Style,
		},
		StatusSeparator: scheme.Table.Border.Style,
		Arrow:           scheme.Table.Border.Style,
		PhaseArrow:      scheme.Chars.PhaseArrowBytes,
		SelectionBg:     scheme.Table.SelectionHighlightBackground.Style.GetBackground(),
	}
}

// Render renders the phase flow and returns the output buffer.
func (p *PhaseFlow) Render(width int) *buffer.LinesBuf {
	spp := p.fleet.CacheStatisticsPerPhase
	if spp == nil || len(p.phases) == 0 {
		p.content.Reset()

		return p.content
	}

	p.data = p.data[:0]

	// Iterate known workflow phases, not map keys: stale ordered-map entries
	// (e.g. a "" phase) would shift the data alignment and corrupt the DONE
	// column.
	for _, ph := range p.phases {
		phaseData := flow.PhaseData{}

		statsPack, ok := spp.Get(ph)
		if ok && statsPack != nil {
			phaseData.Running = len(statsPack[stats.Running])
			phaseData.Failed = len(statsPack[stats.Failed])
		}

		p.data = append(p.data, phaseData)
	}

	// The "DONE" pseudo-phase reads the Done count from the last real workflow
	// phase, not spp.Last(), which would return the wrong phase on extra keys.
	lastPhase := p.phases[len(p.phases)-1]
	lastStats, _ := spp.Get(lastPhase)

	doneData := flow.PhaseData{}
	if lastStats != nil {
		doneData.Done = len(lastStats[stats.Done])
	}

	p.data = append(p.data, doneData)

	p.pf.Width(width).SetDataNoCopy(p.data)

	p.content.Reset()

	p.scheme.Header.Title.Style.RenderLineInto(p.content, phaseFlowHeader)
	p.content.EmptyLine()
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
