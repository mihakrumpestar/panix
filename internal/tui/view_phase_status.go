package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	zone "github.com/lrstanley/bubblezone"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_stats"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"go.uber.org/atomic"
)

const (
	gradientAnimationCycleTime = 4 * time.Second
	phaseArrow                 = "󰜴"
	statusSeparator            = "/"
	animationCacheInterval     = 100 * time.Millisecond
	phaseStatusZonePrefix      = "phase-status"
)

type PhaseStatus struct {
	SelectedPhase int
	Phases        []phases.Phase
	anim          animationState
}

type animationState struct {
	progress    atomic.Uint64
	lastTime    atomic.Int64
	initialized atomic.Bool
}

func NewPhaseStatus() *PhaseStatus {
	return &PhaseStatus{SelectedPhase: -1}
}

func (p *PhaseStatus) Reset() {
	p.SelectedPhase = -1
	p.Phases = nil
}

func (p *PhaseStatus) HandleMouseClick(msg tea.MouseMsg) bool {
	if msg.Action != tea.MouseActionRelease {
		return false
	}
	for i := range p.Phases {
		if z := zone.Get(fmt.Sprintf("%s-%d", phaseStatusZonePrefix, i)); z != nil && z.InBounds(msg) {
			p.SelectedPhase = map[bool]int{true: -1, false: i}[p.SelectedPhase == i]
			return true
		}
	}
	return false
}

func (p *PhaseStatus) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || len(p.Phases) == 0 || p.SelectedPhase < 0 {
		return false
	}
	switch key {
	case "left":
		if p.SelectedPhase > 0 {
			p.SelectedPhase--
		}
		return true
	case "right":
		if p.SelectedPhase < len(p.Phases)-1 {
			p.SelectedPhase++
		}
		return true
	}
	return false
}

func (p *PhaseStatus) GetSelectedPhase() phases.Phase {
	if p.SelectedPhase < 0 || p.SelectedPhase >= len(p.Phases) {
		return ""
	}
	return p.Phases[p.SelectedPhase]
}

func (m *model) ViewPhaseStatus() string {
	return m.conf.ColorScheme.HeaderTitle.Render("=== Phase Status ===") + "\n" + m.renderPhaseFlow()
}

func (m *model) renderPhaseFlow() string {
	phasesList := m.conf.Phases
	if len(phasesList) == 0 {
		return m.conf.ColorScheme.TableRow.Render("No phases to display")
	}

	r := m.resetable.Load()
	r.phaseStatus.Phases = phasesList[:]
	termWidth := r.viewports.ContentWidth()
	stats := r.workflow.State().TargetsLogs.ComputeStatisticsPerPhase()

	row := make([]string, 0, len(phasesList)*2+1)
	for i, phase := range phasesList {
		sp := stats.GetPack(phase)
		if sp == nil {
			sp = &logs_stats.StatsPack{}
		}
		phaseStats := &logs_stats.StatsPack{Running: sp.Running, Failed: sp.Failed}
		row = append(row, m.createPhaseGroup(string(phase), sp.Running, sp.Failed, nil, phaseStats, termWidth, i), phaseArrow)
	}

	lastStats := stats.GetPack(phasesList[len(phasesList)-1])
	if lastStats == nil {
		lastStats = &logs_stats.StatsPack{}
	}
	doneStats := &logs_stats.StatsPack{Done: lastStats.Done}
	row = append(row, m.createPhaseGroup("Done", nil, nil, lastStats.Done, doneStats, termWidth, -1))

	return table.New().
		Width(termWidth).
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(_, col int) lipgloss.Style {
			if (col+1)%2 == 0 {
				return m.conf.ColorScheme.TableBorder.Width(1).Align(lipgloss.Center)
			}
			return lipgloss.NewStyle().Align(lipgloss.Center)
		}).
		Row(row...).String() + "\n"
}

func (m *model) createPhaseGroup(name string, running, failed, done []config_attributes.Xpath, stats *logs_stats.StatsPack, termWidth int, phaseIdx int) string {
	displayName := strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	width := lipgloss.Width(displayName) + 2
	colStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	r := m.resetable.Load()
	phaseName := colStyle.Render(createAnimatedGradient(displayName, stats, m.conf.ColorScheme, &r.phaseStatus.anim))
	statusLine := buildStatusLine(running, failed, done, m.conf.ColorScheme)

	if phaseIdx >= 0 && phaseIdx == r.phaseStatus.SelectedPhase {
		statusLine = m.conf.ColorScheme.SelectionHighlightBackground.Width(width).Align(lipgloss.Center).Render(statusLine)
	} else {
		statusLine = colStyle.Render(statusLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, phaseName, statusLine)
	if phaseIdx >= 0 {
		return zone.Mark(fmt.Sprintf("%s-%d", phaseStatusZonePrefix, phaseIdx), content)
	}
	return content
}

func createAnimatedGradient(text string, stats *logs_stats.StatsPack, colors *config.ColorScheme, anim *animationState) string {
	now := time.Now()

	if !anim.initialized.Load() || now.Sub(time.Unix(0, anim.lastTime.Load())) >= animationCacheInterval {
		anim.initialized.Store(true)
		anim.lastTime.Store(now.UnixNano())

		p := float64(now.UnixNano()%int64(gradientAnimationCycleTime)) / float64(gradientAnimationCycleTime)
		progress := math.Sin(p*2*math.Pi)*0.5 + 0.5

		anim.progress.Store(math.Float64bits(progress))
	}

	progress := math.Float64frombits(anim.progress.Load())

	var state config.PhaseState
	switch {
	case len(stats.Running) > 0:
		state = config.PhaseStateActive
	case len(stats.Failed) > 0:
		state = config.PhaseStateFailed
	case len(stats.Done) > 0:
		state = config.PhaseStateCompleted
	}

	c := colors.PhaseColorPairs[state]
	finalColor := c[0].BlendLuv(c[1], progress)
	return cachedPhaseStyle.Background(lipgloss.Color(finalColor.Hex())).Render(text)
}

var cachedPhaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1)

func buildStatusLine(running, failed, done []config_attributes.Xpath, colors *config.ColorScheme) string {
	var parts []string
	if n := len(running); n > 0 {
		parts = append(parts, colors.StatusRunning.Render(fmt.Sprint(n)))
	}
	if n := len(failed); n > 0 {
		parts = append(parts, colors.StatusError.Render(fmt.Sprint(n)))
	}
	if n := len(done); n > 0 {
		parts = append(parts, colors.StatusOK.Render(fmt.Sprint(n)))
	}
	return strings.Join(parts, colors.TableBorder.Render(statusSeparator))
}
