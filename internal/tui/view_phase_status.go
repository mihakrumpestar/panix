package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// ViewPhaseStatus creates a horizontally oriented visualization of phase status with directional flow
func (m *model) ViewPhaseStatus() string {
	var builder strings.Builder

	colors := config.DefaultColorScheme()
	statePhases := m.workflow.State().Phases

	// Header for the phase status
	builder.WriteString(colors.HeaderTitle.Render("\n=== Phase Status ===\n"))

	// Create horizontal layout with directional arrows
	var phaseNames, runningNumbers, failedNumbers, doneNumbers []string

	for phase, data := range statePhases.Range() {
		if strings.Contains(string(phase), "hook") {
			continue
		}

		// Add right arrow after each phase except the last one
		if phase != phases.Done {
			phase += " →"
		}

		phaseNames = append(phaseNames, string(phase))
		runningNumbers = append(runningNumbers, fmt.Sprintf("🏃 %d", data.Running.Len()))
		failedNumbers = append(failedNumbers, fmt.Sprintf("❌ %d", data.Failed.Len()))
		doneNumbers = append(doneNumbers, fmt.Sprintf("📋 %d", data.Done.Len()))
	}

	// Style for phase names (centered, bold)
	phaseStyle := colors.TableRow.Align(lipgloss.Center).Bold(true)

	// Style for numbers (centered, with colors)
	runningStyle := colors.TableRow.Align(lipgloss.Center).Foreground(lipgloss.Color("#FFA500")) // Orange
	failedStyle := colors.TableRow.Align(lipgloss.Center).Foreground(lipgloss.Color("#FF0000"))  // Red
	totalStyle := colors.TableRow.Align(lipgloss.Center)

	// Create columns with proper spacing
	builder.WriteString("\n")
	for _, name := range phaseNames {
		builder.WriteString(phaseStyle.Render(fmt.Sprintf("%-12s", name)))
	}
	builder.WriteString("\n")

	for _, running := range runningNumbers {
		builder.WriteString(runningStyle.Render(fmt.Sprintf("%-12s", running)))
	}
	builder.WriteString("\n")

	for _, failed := range failedNumbers {
		builder.WriteString(failedStyle.Render(fmt.Sprintf("%-12s", failed)))
	}
	builder.WriteString("\n")

	for _, total := range doneNumbers {
		builder.WriteString(totalStyle.Render(fmt.Sprintf("%-12s", total)))
	}
	builder.WriteString("\n")

	return builder.String()
}
