package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/mihakrumpestar/panix/internal/workflow/tasks"
)

const (
	gradientAnimationCycleTime = 4 * time.Second
)

// PhaseState represents the visual state of a phase
type PhaseState int

const (
	PhaseStateDefault PhaseState = iota
	PhaseStateActive
	PhaseStateFailed
	PhaseStateCompleted
)

// Pre-defined and pre-parsed color pairs for different phase states
var phaseColorPairs = map[PhaseState][2]colorful.Color{
	PhaseStateActive:    {mustColorfullHex("#2952c3"), mustColorfullHex("#3b6bec")}, // Dark blue variations
	PhaseStateFailed:    {mustColorfullHex("#5f1414"), mustColorfullHex("#DC2626")}, // Dark red variations
	PhaseStateCompleted: {mustColorfullHex("#14532D"), mustColorfullHex("#11883d")}, // Dark green variations
	PhaseStateDefault:   {mustColorfullHex("#535862"), mustColorfullHex("#6B7280")}, // Dark gray solid
}

func (m *model) ViewPhaseStatus() string {
	var builder strings.Builder

	colors := m.workflow.State().Conf.Tui.ColorScheme
	statePhases := m.workflow.State().Phases

	builder.WriteString(colors.HeaderTitle.Render("=== Phase Status ===\n"))
	builder.WriteString(renderPhaseFlow(statePhases, colors, m.modelView.dimensions.Width))

	return builder.String()
}

func renderPhaseFlow(statePhases *phases.PhaseStates, colors *config.ColorScheme, termWidth int) string {
	// Collect phases first to determine count
	phases := make([]phases.Phase, 0)
	for phase := range statePhases.Range() {
		if !strings.Contains(string(phase), "hook") {
			phases = append(phases, phase)
		}
	}

	if len(phases) == 0 {
		return colors.TableRow.Render("No phases to display")
	}

	// Create table for phase flow
	t := table.New().
		Width(termWidth - 2).
		Border(lipgloss.HiddenBorder()). // Debug: lipgloss.NormalBorder()
		StyleFunc(func(row, col int) lipgloss.Style {
			if (col+1)%2 == 0 {
				return colors.TableBorder.Width(1).Align(lipgloss.Center)
			}

			return lipgloss.NewStyle().Align(lipgloss.Center)
		})

	// Build table row with phase groups and arrows
	var row []string

	for i, phase := range phases {
		// Create phase group
		phaseGroup := createPhaseGroup(phase, statePhases.Value(phase), colors, termWidth)
		row = append(row, phaseGroup)

		// Add arrow except for last phase
		if i < len(phases)-1 {
			row = append(row, "󰜴")
		}
	}

	// Set the table row
	t.Row(row...)

	return fmt.Sprintf("%s\n", t.String())
}

func createPhaseGroup(phase phases.Phase, data *tasks.PhaseTasksStates, colors *config.ColorScheme, termWidth int) string {
	phaseName := string(phase)
	displayName := strings.Title(phaseName)
	running := data.LenByState(tasks.Running)
	failed := data.LenByState(tasks.Failed)
	done := data.LenByState(tasks.Done)

	// Create phase name with gradient
	phaseNameText := createAnimatedGradient(displayName, running, failed, done)
	columnStyle := lipgloss.NewStyle().Width(lipgloss.Width(phaseNameText)).Margin(int(float64(termWidth) * 0.005)).Align(lipgloss.Center)
	phaseNameText = columnStyle.Render(phaseNameText)

	// Create status line
	statusLine := buildStatusLine(running, failed, done, colors)
	statusLine = columnStyle.Render(statusLine)
	// Both name and stats should be centered within their container
	phaseGroupContent := lipgloss.JoinVertical(lipgloss.Center, phaseNameText, statusLine)

	return phaseGroupContent
}

func createAnimatedGradient(text string, running, failed, done int) string {
	phaseState := determinePhaseState(running, failed, done)

	now := time.Now()
	progress := float64(now.UnixNano()%int64(gradientAnimationCycleTime)) / float64(gradientAnimationCycleTime)

	// Create a sine wave for smooth back-and-forth animation
	// This goes from 0 to 1 and back to 0 smoothly
	blendFactor := math.Sin(progress*2*math.Pi)*0.5 + 0.5

	colorPair := phaseColorPairs[phaseState]
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
func buildStatusLine(running, failed, done int, colors *config.ColorScheme) string {
	indicators := make([]string, 0)

	if running > 0 {
		indicators = append(indicators, colors.StatusRunning.Render(fmt.Sprintf("%d", running)))
	}

	if failed > 0 {
		indicators = append(indicators, colors.StatusError.Render(fmt.Sprintf("%d", failed)))
	}

	if done > 0 {
		indicators = append(indicators, colors.StatusOK.Render(fmt.Sprintf("%d", done)))
	}

	statusText := strings.Join(indicators, colors.TableBorder.Render("/"))

	return statusText
}

// Helpers

func mustColorfullHex(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		panic(fmt.Sprintf("Invalid color hex %s: %v", hex, err))
	}
	return c
}

// determinePhaseState determines the visual state of a phase based on its counts
func determinePhaseState(running, failed, done int) PhaseState {
	if running > 0 {
		return PhaseStateActive
	}
	if failed > 0 && running == 0 && done == 0 {
		return PhaseStateFailed
	}
	if done > 0 && running == 0 && failed == 0 {
		return PhaseStateCompleted
	}

	return PhaseStateDefault
}
