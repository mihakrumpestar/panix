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
)

func (m *model) ViewPhaseStatus() string {
	var builder strings.Builder

	colors := m.workflow.State().Conf.ColorScheme

	builder.WriteString(colors.HeaderTitle.Render("=== Phase Status ===\n"))
	builder.WriteString(m.renderPhaseFlow(colors))

	return builder.String()
}

func (m *model) renderPhaseFlow(colors *config.ColorScheme) string {
	phasesList := m.workflow.State().Phases

	if len(phasesList) == 0 {
		return colors.TableRow.Render("No phases to display")
	}

	termWidth := m.modelView.viewports.ContentWidth()

	// Create table for phase flow
	t := table.New().
		Width(termWidth).
		Border(lipgloss.HiddenBorder()). // Debug: lipgloss.NormalBorder()
		StyleFunc(func(row, col int) lipgloss.Style {
			if (col+1)%2 == 0 {
				return colors.TableBorder.Width(1).Align(lipgloss.Center)
			}

			return lipgloss.NewStyle().Align(lipgloss.Center)
		})

	stats := m.workflow.State().Conf.TargetsLogs.ComputeStatisticsPerPhase()

	// Build table row with phase groups and arrows
	var row []string

	for _, phase := range phasesList {
		statsPhase := stats.GetPack(phase)
		statsPhase.Done = []config_attributes.Xpath{}

		// Create phase group
		phaseGroup := createPhaseGroup(string(phase), statsPhase, colors, termWidth)
		row = append(row, phaseGroup)

		// Add arrow
		row = append(row, "󰜴")
	}

	// Add "Done" phase group

	statsDone := stats.GetPack(phasesList[len(phasesList)-1])
	statsDone.Running = []config_attributes.Xpath{}
	statsDone.Failed = []config_attributes.Xpath{}
	phaseGroup := createPhaseGroup("Done", statsDone, colors, termWidth)
	row = append(row, phaseGroup)

	// Set the table row
	t.Row(row...)

	return fmt.Sprintf("%s\n", t.String())
}

func createPhaseGroup(phase string, stats *logs_stats.StatsPack, colors *config.ColorScheme, termWidth int) string {
	displayName := strings.Title(phase)

	// Create phase name with gradient
	phaseNameText := createAnimatedGradient(displayName, stats, colors)
	columnStyle := lipgloss.NewStyle().Width(lipgloss.Width(phaseNameText)).Margin(int(float64(termWidth) * 0.005)).Align(lipgloss.Center)
	phaseNameText = columnStyle.Render(phaseNameText)

	// Create status line
	statusLine := buildStatusLine(stats, colors)
	statusLine = columnStyle.Render(statusLine)
	// Both name and stats should be centered within their container
	phaseGroupContent := lipgloss.JoinVertical(lipgloss.Center, phaseNameText, statusLine)

	return phaseGroupContent
}

func createAnimatedGradient(text string, stats *logs_stats.StatsPack, colors *config.ColorScheme) string {
	phaseState := determinePhaseState(stats)

	now := time.Now()
	progress := float64(now.UnixNano()%int64(gradientAnimationCycleTime)) / float64(gradientAnimationCycleTime)

	// Create a sine wave for smooth back-and-forth animation
	// This goes from 0 to 1 and back to 0 smoothly
	blendFactor := math.Sin(progress*2*math.Pi)*0.5 + 0.5

	colorPair := colors.PhaseColorPairs[phaseState]
	c1 := colorPair[0]
	c2 := colorPair[1]
	finalColor := c1.BlendLuv(c2, blendFactor)

	styledText := lipgloss.NewStyle().
		Background(lipgloss.Color(finalColor.Hex())).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1).
		Render(text)

	return styledText
}

// buildStatusLine creates compact status indicators for horizontal layout
func buildStatusLine(stats *logs_stats.StatsPack, colors *config.ColorScheme) string {
	indicators := make([]string, 0)

	if len(stats.Running) > 0 {
		indicators = append(indicators, colors.StatusRunning.Render(fmt.Sprintf("%d", len(stats.Running))))
	}

	if len(stats.Failed) > 0 {
		indicators = append(indicators, colors.StatusError.Render(fmt.Sprintf("%d", len(stats.Failed))))
	}

	if len(stats.Done) > 0 {
		indicators = append(indicators, colors.StatusOK.Render(fmt.Sprintf("%d", len(stats.Done))))
	}

	statusText := strings.Join(indicators, colors.TableBorder.Render("/"))

	return statusText
}

// Helpers

// determinePhaseState determines the visual state of a phase based on its counts
func determinePhaseState(stats *logs_stats.StatsPack) config.PhaseState {
	if len(stats.Running) > 0 {
		return config.PhaseStateActive
	}
	if len(stats.Failed) > 0 && len(stats.Running) == 0 && len(stats.Done) == 0 {
		return config.PhaseStateFailed
	}
	if len(stats.Done) > 0 && len(stats.Running) == 0 && len(stats.Failed) == 0 {
		return config.PhaseStateCompleted
	}

	return config.PhaseStateDefault
}
