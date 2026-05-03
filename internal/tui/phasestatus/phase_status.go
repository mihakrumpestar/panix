package phasestatus

import (
	"fmt"
	"hash/fnv"
	"maps"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/logs/stats"
	"github.com/mihakrumpestar/panix/internal/pkg/cache"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/render"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
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

type phaseStatusCacheKey struct {
	statisticsHash uint64
	width          int
	selectedIndex  int
}

type PhaseStatus struct {
	Selected Selected `json:"selected"`

	CacheStatisticsPerPhase *stats.StatisticsPerPhase                `json:"-"`
	animation               animationState                           `json:"-"`
	cache                   cache.Cache[string, phaseStatusCacheKey] `json:"-"`
	lastRenderWidth         int                                      `json:"-"`
}

func NewPhaseStatus() *PhaseStatus {
	return &PhaseStatus{
		Selected: Selected{Index: -1},
	}
}

type Selected struct {
	Phase phase.Phase `json:"phase,omitempty"`
	Index int         `json:"index"`
}

type animationState struct {
	progress atomic.Uint64
	lastTime atomic.Time
}

func (p *PhaseStatus) Reset() {
	p.Selected.Phase = ""
	p.Selected.Index = -1
}

func (p *PhaseStatus) HandleMouseClick(msg render.MouseClickMsg) bool {
	lines := render.CurrentLines()
	if msg.Y < 0 || msg.Y >= len(lines) {
		return false
	}

	for idx := range p.CacheStatisticsPerPhase.Len() {
		zoneName := fmt.Sprintf("%s-%d", phaseStatusZonePrefix, idx)
		if render.IsZoneAtLine(lines[msg.Y], msg.X, zoneName) {
			if p.Selected.Index == idx {
				p.Selected.Index = -1
				p.Selected.Phase = ""
			} else {
				p.Selected.Index = idx
				p.applyIndexToPhase()
			}

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
		phaseStatusCacheKey{
			statisticsHash: hashStatisticsPerPhase(p.CacheStatisticsPerPhase),
			width:          width,
			selectedIndex:  p.Selected.Index,
		})
}

func hashStatisticsPerPhase(spp *stats.StatisticsPerPhase) uint64 {
	if spp == nil {
		return 0
	}

	hash := fnv.New64a()

	for _, pair := range spp.Pairs() {
		_, _ = hash.Write([]byte(pair.Key))

		for state, xpaths := range pair.Value {
			_, _ = hash.Write([]byte(state))

			for _, xp := range xpaths {
				_, _ = hash.Write([]byte(xp))
			}
		}
	}

	return hash.Sum64()
}

// Helpers

func (p *PhaseStatus) applyIndexToPhase() {
	p.Selected.Phase = p.CacheStatisticsPerPhase.Pairs()[p.Selected.Index].Key
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
		return render.Mark(fmt.Sprintf("%s-%d", phaseStatusZonePrefix, phaseIdx), content)
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

	return colors.PhaseStatus.Pill.Background(lipgloss.Color(finalColor.Hex())).Render(text)
}

func buildStatusLine(statsPack stats.StatsPack, colors *colorscheme.ColorScheme) string {
	var parts []string

	num := len(statsPack[stats.Running])
	if num > 0 {
		parts = append(parts, colors.Status.Running.Render(strconv.Itoa(num)))
	}

	num = len(statsPack[stats.Failed])
	if num > 0 {
		parts = append(parts, colors.Status.Failed.Render(strconv.Itoa(num)))
	}

	num = len(statsPack[stats.Done])
	if num > 0 {
		parts = append(parts, colors.Status.OK.Render(strconv.Itoa(num)))
	}

	return strings.Join(parts, colors.Table.Border.Render(statusSeparator))
}
