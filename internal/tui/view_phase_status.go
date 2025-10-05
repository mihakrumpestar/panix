package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func (m *model) ViewPhaseStatus() string {
	var builder strings.Builder

	colors := config.DefaultColorScheme()
	statePhases := m.workflow.State().Phases

	builder.WriteString(colors.HeaderTitle.Render("\n=== Phase Status ===\n"))

	phaseData := collectPhaseData(statePhases)

	builder.WriteString(renderPhaseFlow(phaseData, colors))

	return builder.String()
}

// PhaseData holds all information needed for rendering a phase
type PhaseData struct {
	Name        string
	DisplayName string
	Running     int
	Failed      int
	Done        int
	IsActive    bool
	IsFailed    bool
	IsCompleted bool
}

// collectPhaseData gathers and processes phase information
func collectPhaseData(statePhases *phases.PhaseStates) []PhaseData {
	var phasesData []PhaseData

	for phase, data := range statePhases.Range() {
		if strings.Contains(string(phase), "hook") {
			continue
		}

		running := data.Running.Len()
		failed := data.Failed.Len()
		done := data.Done.Len()

		phaseData := PhaseData{
			Name:        string(phase),
			DisplayName: strings.Title(string(phase)),
			Running:     running,
			Failed:      failed,
			Done:        done,
			IsActive:    running > 0,
			IsFailed:    failed > 0 && running == 0 && done == 0,
			IsCompleted: done > 0 && running == 0 && failed == 0,
		}

		phasesData = append(phasesData, phaseData)
	}

	return phasesData
}

// renderPhaseFlow creates the main phase flow visualization in horizontal layout
func renderPhaseFlow(phasesData []PhaseData, colors config.ColorScheme) string {
	if len(phasesData) == 0 {
		return colors.TableRow.Render("No phases to display")
	}

	// Build phase groups
	var phaseGroups []string

	for i, phase := range phasesData {
		// Create phase group with proper structure
		phaseGroup := createPhaseGroup(phase, colors)

		// Add large arrow except for last phase
		if i < len(phasesData)-1 {
			arrow := createLargeArrow(colors)
			phaseGroup = lipgloss.JoinHorizontal(lipgloss.Left, phaseGroup, arrow)
		}

		phaseGroups = append(phaseGroups, phaseGroup)
	}

	// Join all phase groups horizontally
	return fmt.Sprintf("\n%s\n", lipgloss.JoinHorizontal(lipgloss.Left, phaseGroups...))
}

// createPhaseGroup creates a vertical group with phase name and stats
func createPhaseGroup(phase PhaseData, colors config.ColorScheme) string {
	style := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(16)

	centeredPhaseName := style.Render(createAnimatedGradient(phase.DisplayName, phase, colors))
	centeredStats := style.Render(buildStatusLine(phase, colors))

	return lipgloss.JoinVertical(lipgloss.Left, centeredPhaseName, centeredStats)
}

// createLargeArrow creates a large arrow for connecting phases
func createLargeArrow(colors config.ColorScheme) string {
	arrowStyle := colors.TableBorder.
		Bold(true).
		Padding(0, 1).
		Align(lipgloss.Center)

	return arrowStyle.Render("󰜴")
}

// createAnimatedGradient creates a simple gradient background using 2 darker colors
func createAnimatedGradient(text string, phase PhaseData, colors config.ColorScheme) string {
	// Get 2 darker colors for this phase state
	var color1, color2 string
	if phase.IsActive {
		color1, color2 = "#2952c3", "#3b6bec" // Dark blue variations
	} else if phase.IsFailed {
		color1, color2 = "#7F1D1D", "#DC2626" // Dark red variations
	} else if phase.IsCompleted {
		color1, color2 = "#14532D", "#16A34A" // Dark green variations
	} else {
		color1, color2 = "#6B7280", "#6B7280" // Dark gray solid
	}

	// Parse colors using colorful library
	c1, err := colorful.Hex(color1)
	if err != nil {
		panic(err)
	}

	c2, err := colorful.Hex(color2)
	if err != nil {
		panic(err)
	}

	// Create fluid animated gradient that goes back and forth
	now := time.Now()
	// Use a 4-second cycle for smoother animation
	cycleTime := 4 * time.Second
	progress := float64(now.UnixNano()%int64(cycleTime)) / float64(cycleTime)

	// Create a sine wave for smooth back-and-forth animation
	// This goes from 0 to 1 and back to 0 smoothly
	blendFactor := math.Sin(progress*2*math.Pi)*0.5 + 0.5

	// Blend between the two colors based on the smooth animation
	finalColor := c1.BlendLuv(c2, blendFactor)

	// Apply the gradient background to the text with white text for contrast
	styledText := lipgloss.NewStyle().
		Background(lipgloss.Color(finalColor.Hex())).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1).
		Render(text)

	return styledText
}

// buildStatusLine creates compact status indicators for horizontal layout
func buildStatusLine(phase PhaseData, colors config.ColorScheme) string {
	var indicators []string

	if phase.Running > 0 {
		indicators = append(indicators, colors.StatusRunning.Render(fmt.Sprintf("%d", phase.Running)))
	}

	if phase.Failed > 0 {
		indicators = append(indicators, colors.StatusError.Render(fmt.Sprintf("%d", phase.Failed)))
	}

	if phase.Done > 0 {
		indicators = append(indicators, colors.StatusOK.Render(fmt.Sprintf("%d", phase.Done)))
	}

	statusText := strings.Join(indicators, colors.TableBorder.Render("/"))
	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(statusText)
}
