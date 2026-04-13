package phasestatus

import (
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
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
	Selected Selected `json:"selected"`

	CacheStatisticsPerPhase *stats.StatisticsPerPhase `json:"-"`
	animation               animationState            `json:"-"`
	cache                   cache.Cache[string]       `json:"-"`
	lastRenderWidth         int                       `json:"-"`
}

func NewPhaseStatus() *PhaseStatus {
	return &PhaseStatus{
		Selected: Selected{Index: -1},
	}
}

type Selected struct {
	Phase phases.Phase `json:"phase"`
	Index int          `json:"index"`
}

type animationState struct {
	progress atomic.Uint64
	lastTime atomic.Time
}

func (p *PhaseStatus) Reset() {
	p.Selected.Phase = ""
	p.Selected.Index = -1
}

func (p *PhaseStatus) HandleMouseClick(msg tea.MouseClickMsg) bool {
	for i := range p.CacheStatisticsPerPhase.Len() {
		if z := zone.Get(fmt.Sprintf("%s-%d", phaseStatusZonePrefix, i)); z != nil && z.InBounds(msg) {
			p.Selected.Index = map[bool]int{true: -1, false: i}[p.Selected.Index == i]

			p.applyIndexToPhase()

			return true
		}
	}

	return false
}

func (p *PhaseStatus) HandleNavigation(key string, hasActiveInnerViewport bool) bool {
	if hasActiveInnerViewport || p.CacheStatisticsPerPhase.Len() == 0 || p.Selected.Index < 0 {
		return false
	}

	switch key {
	case "left":
		if p.Selected.Index > 0 {
			p.Selected.Index--
			p.applyIndexToPhase()

			return true
		}
	case "right":
		if p.Selected.Index < p.CacheStatisticsPerPhase.Len()-1 {
			p.Selected.Index++
			p.applyIndexToPhase()

			return true
		}
	}

	return false
}

func (p *PhaseStatus) applyIndexToPhase() {
	p.Selected.Phase = p.CacheStatisticsPerPhase.Pairs()[p.Selected.Index].Key
}

func (p *PhaseStatus) View(width int, colorScheme *colorscheme.ColorScheme) string {
	widthChanged := width != p.lastRenderWidth
	if widthChanged {
		p.lastRenderWidth = width
	}

	return p.cache.Get(
		func() (string, bool) {
			if !widthChanged && !p.animationNeedsUpdate() {
				return "", false
			}

			return p.renderPhaseFlow(width, colorScheme), true
		},
		p.CacheStatisticsPerPhase, width, p.Selected.Index)
}

func (p *PhaseStatus) renderPhaseFlow(width int, colorScheme *colorscheme.ColorScheme) string {
	result := colorScheme.Header.Title.Render("=== Phase Status ===") + "\n"

	row := p.buildPhaseRows(colorScheme)
	result += table.New().Width(width).Border(lipgloss.HiddenBorder()).
		StyleFunc(func(_, col int) lipgloss.Style {
			if (col+1)%2 == 0 {
				return colorScheme.Table.Border.Width(1).Align(lipgloss.Center)
			}

			return lipgloss.NewStyle().Align(lipgloss.Center)
		}).Row(row...).String() + "\n"

	return result
}

func (p *PhaseStatus) buildPhaseRows(colorScheme *colorscheme.ColorScheme) []string {
	statistics := p.CacheStatisticsPerPhase

	row := make([]string, 0, statistics.Len()*2+1)

	for idx, pair := range statistics.Pairs() {
		statsPack := make(stats.StatsPack)

		if pair.Value != nil {
			maps.Copy(statsPack, pair.Value)
			delete(statsPack, stats.Done)
		}

		row = append(row,
			p.createPhaseGroup(
				pair.Key.String(),
				statsPack,
				idx,
				colorScheme,
			),
			phaseArrow,
		)
	}

	statsPack, _ := statistics.Last()

	tpmStatPack := stats.StatsPack{
		stats.Done: statsPack.Value[stats.Done],
	}

	row = append(row, p.createPhaseGroup("Done", tpmStatPack, -1, colorScheme))

	return row
}

func (p *PhaseStatus) animationNeedsUpdate() bool {
	lastTime := p.animation.lastTime.Load()
	if lastTime.IsZero() {
		return true
	}

	return time.Since(lastTime) >= animationCacheInterval
}

func (p *PhaseStatus) createPhaseGroup(name string, statsPack stats.StatsPack, phaseIdx int, colorScheme *colorscheme.ColorScheme) string {
	displayName := strings.ToTitle(name)
	width := lipgloss.Width(displayName) + displayNamePadding
	colStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)

	phaseName := colStyle.Render(p.createAnimatedGradient(displayName, statsPack, colorScheme))
	statusLine := buildStatusLine(statsPack, colorScheme)

	if phaseIdx >= 0 && phaseIdx == p.Selected.Index {
		statusLine = colorScheme.Table.SelectionHighlightBackground.Width(width).Align(lipgloss.Center).Render(statusLine)
	} else {
		statusLine = colStyle.Render(statusLine)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, phaseName, statusLine)
	if phaseIdx >= 0 {
		return zone.Mark(fmt.Sprintf("%s-%d", phaseStatusZonePrefix, phaseIdx), content)
	}

	return content
}

func (p *PhaseStatus) createAnimatedGradient(text string, statsPack stats.StatsPack, colors *colorscheme.ColorScheme) string {
	now := time.Now()

	lastTime := p.animation.lastTime.Load()
	if lastTime.IsZero() || now.Sub(lastTime) >= animationCacheInterval {
		p.animation.lastTime.Store(now)

		nowNano := now.UnixNano()
		tmp := float64(nowNano%int64(gradientAnimationCycleTime)) / float64(gradientAnimationCycleTime)
		progress := math.Sin(tmp*2*math.Pi)*animationAmplitude + animationAmplitude

		p.animation.progress.Store(math.Float64bits(progress))
	}

	progress := math.Float64frombits(p.animation.progress.Load())

	var gradient [2]colorful.Color

	switch {
	case len(statsPack[stats.Running]) > 0:
		gradient = colors.PhaseStatus.Running
	case len(statsPack[stats.Failed]) > 0:
		gradient = colors.PhaseStatus.Failed
	case len(statsPack[stats.Done]) > 0:
		gradient = colors.PhaseStatus.Done
	default:
		gradient = colors.PhaseStatus.Default
	}

	finalColor := gradient[0].BlendLuv(gradient[1], progress)

	return cachedPhaseStyle.Background(lipgloss.Color(finalColor.Hex())).Render(text)
}

var cachedPhaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1)

func buildStatusLine(statsPack stats.StatsPack, colors *colorscheme.ColorScheme) string {
	var parts []string

	if n := len(statsPack[stats.Running]); n > 0 {
		parts = append(parts, colors.Status.Running.Render(strconv.Itoa(n)))
	}

	if n := len(statsPack[stats.Failed]); n > 0 {
		parts = append(parts, colors.Status.Failed.Render(strconv.Itoa(n)))
	}

	if n := len(statsPack[stats.Done]); n > 0 {
		parts = append(parts, colors.Status.OK.Render(strconv.Itoa(n)))
	}

	return strings.Join(parts, colors.Table.Border.Render(statusSeparator))
}
