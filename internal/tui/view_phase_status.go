package tui

import (
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/stats"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"go.uber.org/atomic"
)

const (
	gradientAnimationCycleTime = 4 * time.Second
	phaseArrow                 = "󰜴"
	statusSeparator            = "/"
	animationCacheInterval     = 100 * time.Millisecond
	phaseStatusZonePrefix      = "phase-status"

	displayNamePadding = 2
	animationAmplitude = 0.5
)

type PhaseStatus struct {
	SelectedPhase int
	Phases        []phases.Phase
	anim          animationState
	cache         phaseStatusCache
}

type phaseStatusCache struct {
	width         int
	selectedPhase int
	stateHash     uint64
	rendered      string
}

type animationState struct {
	progress atomic.Uint64
	lastTime atomic.Time
}

func NewPhaseStatus() *PhaseStatus {
	return &PhaseStatus{SelectedPhase: -1}
}

func (p *PhaseStatus) Reset() {
	p.SelectedPhase = -1
	p.Phases = nil
}

func (p *PhaseStatus) HandleMouseClick(msg tea.MouseClickMsg) bool {
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
	return m.conf.ColorScheme.Header.Title.Render("=== Phase Status ===") + "\n" + m.renderPhaseFlow()
}

func (m *model) renderPhaseFlow() string {
	phasesList := m.conf.Phases
	if len(phasesList) == 0 {
		return m.conf.ColorScheme.Table.Row.Render("No phases to display")
	}

	resetable := m.resetable.Load()
	resetable.phaseStatus.Phases = phasesList
	termWidth := resetable.viewports.ContentWidth()
	statistics := resetable.workflow.State().TargetsLogs.ComputeStatisticsPerPhase(phasesList)
	stateHash := computeStateHash(phasesList, statistics)

	if !m.animationNeedsUpdate(&resetable.phaseStatus.anim) && resetable.phaseStatus.cache.width == termWidth &&
		resetable.phaseStatus.cache.selectedPhase == resetable.phaseStatus.SelectedPhase && resetable.phaseStatus.cache.stateHash == stateHash {
		return resetable.phaseStatus.cache.rendered
	}

	row := m.buildPhaseRows(phasesList, statistics)
	result := table.New().Width(termWidth).Border(lipgloss.HiddenBorder()).
		StyleFunc(func(_, col int) lipgloss.Style {
			if (col+1)%2 == 0 {
				return m.conf.ColorScheme.Table.Border.Width(1).Align(lipgloss.Center)
			}

			return lipgloss.NewStyle().Align(lipgloss.Center)
		}).Row(row...).String() + "\n"

	phaseStatus := resetable.phaseStatus

	phaseStatus.cache = phaseStatusCache{
		width:         termWidth,
		selectedPhase: phaseStatus.SelectedPhase,
		stateHash:     stateHash,
		rendered:      result,
	}

	return result
}

func (m *model) buildPhaseRows(phasesList []phases.Phase, statistics *stats.StatisticsPerPhase) []string {
	row := make([]string, 0, len(phasesList)*2+1)

	for idx, phase := range phasesList {
		statpack := statistics.GetPack(phase)
		if statpack == nil {
			statpack = &stats.StatsPack{}
		}

		row = append(row, m.createPhaseGroup(
			string(phase),
			statpack.Running,
			statpack.Failed,
			nil,
			&stats.StatsPack{Running: statpack.Running, Failed: statpack.Failed},
			idx),
			phaseArrow)
	}

	ls := statistics.GetPack(phasesList[len(phasesList)-1])
	if ls == nil {
		ls = &stats.StatsPack{}
	}

	row = append(row, m.createPhaseGroup("Done", nil, nil, ls.Done, &stats.StatsPack{Done: ls.Done}, -1))

	return row
}

func computeStateHash(phasesList []phases.Phase, statistics *stats.StatisticsPerPhase) uint64 {
	hash := fnv.New64a()

	for _, phase := range phasesList {
		statsPack := statistics.GetPack(phase)
		if statsPack != nil {
			_, _ = fmt.Fprintf(hash, "%v", statsPack)
		}
	}

	return hash.Sum64()
}

func (m *model) animationNeedsUpdate(anim *animationState) bool {
	lastTime := anim.lastTime.Load()
	if lastTime.IsZero() {
		return true
	}

	return time.Since(lastTime) >= animationCacheInterval
}

func (m *model) createPhaseGroup(name string, running, failed, done []attributes.Xpath, stats *stats.StatsPack, phaseIdx int) string {
	if len(name) == 0 {
		name = "unnamed"
	}

	displayName := strings.ToTitle(name)
	width := lipgloss.Width(displayName) + displayNamePadding
	colStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	r := m.resetable.Load()
	phaseName := colStyle.Render(createAnimatedGradient(displayName, stats, m.conf.ColorScheme, &r.phaseStatus.anim))
	statusLine := buildStatusLine(running, failed, done, m.conf.ColorScheme)

	if phaseIdx >= 0 && phaseIdx == r.phaseStatus.SelectedPhase {
		statusLine = m.conf.ColorScheme.Table.SelectionHighlightBackground.Width(width).Align(lipgloss.Center).Render(statusLine)
	} else {
		statusLine = colStyle.Render(statusLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, phaseName, statusLine)
	if phaseIdx >= 0 {
		return zone.Mark(fmt.Sprintf("%s-%d", phaseStatusZonePrefix, phaseIdx), content)
	}

	return content
}

func createAnimatedGradient(text string, stats *stats.StatsPack, colors *config.ColorScheme, anim *animationState) string {
	now := time.Now()

	lastTime := anim.lastTime.Load()
	if lastTime.IsZero() || now.Sub(lastTime) >= animationCacheInterval {
		anim.lastTime.Store(now)

		nowNano := now.UnixNano()
		p := float64(nowNano%int64(gradientAnimationCycleTime)) / float64(gradientAnimationCycleTime)
		progress := math.Sin(p*2*math.Pi)*animationAmplitude + animationAmplitude

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

func buildStatusLine(running, failed, done []attributes.Xpath, colors *config.ColorScheme) string {
	var parts []string

	if n := len(running); n > 0 {
		parts = append(parts, colors.Status.Running.Render(strconv.Itoa(n)))
	}

	if n := len(failed); n > 0 {
		parts = append(parts, colors.Status.Error.Render(strconv.Itoa(n)))
	}

	if n := len(done); n > 0 {
		parts = append(parts, colors.Status.OK.Render(strconv.Itoa(n)))
	}

	return strings.Join(parts, colors.Table.Border.Render(statusSeparator))
}
