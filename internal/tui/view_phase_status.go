package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_stats"
)

const (
	gradientAnimationCycleTime = 4 * time.Second
	phaseArrow                 = "󰜴"
	statusSeparator            = "/"
	animationCacheInterval     = 50 * time.Millisecond
)

// ViewPhaseStatus renders the phase status view.
func (m *model) ViewPhaseStatus() string {
	colors := m.workflow.State().Conf.ColorScheme

	var b strings.Builder
	b.WriteString(colors.HeaderTitle.Render("=== Phase Status ===\n"))
	b.WriteString(m.renderPhaseFlow(colors))

	return b.String()
}

func (m *model) renderPhaseFlow(colors *config.ColorScheme) string {
	phasesList := m.workflow.State().Phases

	if len(phasesList) == 0 {
		return colors.TableRow.Render("No phases to display")
	}

	termWidth := m.modelView.viewports.ContentWidth()

	// Cache styles to avoid repeated allocations
	centerStyle := lipgloss.NewStyle().Align(lipgloss.Center)
	borderStyle := colors.TableBorder.Width(1).Align(lipgloss.Center)

	// Create table for phase flow
	t := table.New().
		Width(termWidth).
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			if (col+1)%2 == 0 {
				return borderStyle
			}
			return centerStyle
		})

	stats := m.workflow.State().Conf.TargetsLogs.ComputeStatisticsPerPhase()

	// Build table row with phase groups and arrows
	row := make([]string, 0, len(phasesList)*2+1)

	for _, phase := range phasesList {
		statsPack := stats.GetPack(phase)
		if statsPack == nil {
			statsPack = &logs_stats.StatsPack{}
		}

		// For active phases, don't show Done items and don't use Done for color calc
		phaseGroup := createPhaseGroup(
			string(phase),
			statsPack.Running,
			statsPack.Failed,
			nil, // Done is nil for active phases
			&logs_stats.StatsPack{Running: statsPack.Running, Failed: statsPack.Failed},
			colors,
			termWidth,
		)
		row = append(row, phaseGroup, phaseArrow)
	}

	// Add "Done" phase group - for final phase, don't show Running/Failed
	lastPhase := phasesList[len(phasesList)-1]
	statsDone := stats.GetPack(lastPhase)
	if statsDone == nil {
		statsDone = &logs_stats.StatsPack{}
	}

	donePhaseGroup := createPhaseGroup(
		"Done",
		nil, // Running is nil for Done phase
		nil, // Failed is nil for Done phase
		statsDone.Done,
		&logs_stats.StatsPack{Done: statsDone.Done},
		colors,
		termWidth,
	)
	row = append(row, donePhaseGroup)

	t.Row(row...)

	return t.String() + "\n"
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func createPhaseGroup(
	phase string,
	running, failed, done []config_attributes.Xpath,
	stats *logs_stats.StatsPack,
	colors *config.ColorScheme,
	termWidth int,
) string {
	displayName := capitalize(phase)

	// Compute width from plain text before styling to avoid expensive lipgloss.Width
	plainNameWidth := lipgloss.Width(displayName) + 2 // +2 for padding
	columnStyle := lipgloss.NewStyle().Width(plainNameWidth).Margin(int(float64(termWidth) * 0.005)).Align(lipgloss.Center)

	// Create phase name with gradient
	phaseNameText := createAnimatedGradient(displayName, stats, colors)
	phaseNameText = columnStyle.Render(phaseNameText)

	// Create status line
	statusLine := buildStatusLine(running, failed, done, colors)
	statusLine = columnStyle.Render(statusLine)

	return lipgloss.JoinVertical(lipgloss.Center, phaseNameText, statusLine)
}

// Animation state - updated at most once per cache interval
type animationState struct {
	progress    float64
	lastTime    time.Time
	initialized bool
}

var globalAnimation = &animationState{}

func createAnimatedGradient(text string, stats *logs_stats.StatsPack, colors *config.ColorScheme) string {
	phaseState := determinePhaseState(stats)

	// Update animation state at most once per cache interval
	now := time.Now()
	if !globalAnimation.initialized || now.Sub(globalAnimation.lastTime) >= animationCacheInterval {
		if !globalAnimation.initialized {
			globalAnimation.initialized = true
		}
		globalAnimation.lastTime = now
		progress := float64(now.UnixNano()%int64(gradientAnimationCycleTime)) / float64(gradientAnimationCycleTime)
		globalAnimation.progress = math.Sin(progress*2*math.Pi)*0.5 + 0.5
	}

	colorPair := colors.PhaseColorPairs[phaseState]
	finalColor := colorPair[0].BlendLuv(colorPair[1], globalAnimation.progress)

	return cachedPhaseStyle.Background(lipgloss.Color(finalColor.Hex())).Render(text)
}

// Reusable style for phase rendering - only the background color changes
var cachedPhaseStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF")).
	Bold(true).
	Padding(0, 1)

// buildStatusLine creates compact status indicators for horizontal layout
func buildStatusLine(running, failed, done []config_attributes.Xpath, colors *config.ColorScheme) string {
	var indicators []string

	if len(running) > 0 {
		indicators = append(indicators, colors.StatusRunning.Render(fmt.Sprintf("%d", len(running))))
	}

	if len(failed) > 0 {
		indicators = append(indicators, colors.StatusError.Render(fmt.Sprintf("%d", len(failed))))
	}

	if len(done) > 0 {
		indicators = append(indicators, colors.StatusOK.Render(fmt.Sprintf("%d", len(done))))
	}

	return strings.Join(indicators, colors.TableBorder.Render(statusSeparator))
}

// determinePhaseState determines the visual state of a phase based on its counts
func determinePhaseState(stats *logs_stats.StatsPack) config.PhaseState {
	switch {
	case len(stats.Running) > 0:
		return config.PhaseStateActive
	case len(stats.Failed) > 0 && len(stats.Running) == 0 && len(stats.Done) == 0:
		return config.PhaseStateFailed
	case len(stats.Done) > 0 && len(stats.Running) == 0 && len(stats.Failed) == 0:
		return config.PhaseStateCompleted
	default:
		return config.PhaseStateDefault
	}
}
