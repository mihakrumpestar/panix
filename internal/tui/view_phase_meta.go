package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

// Render generates the Docker-style build log view with tree structure
func (m *model) PrintBuildLogs() string {
	var builder strings.Builder

	// Header for the log view
	builder.WriteString("\n" + lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ADD8")).
		Render("=== Build Logs ===\n"))

	for flakeName, flake := range m.state.Conf.Flakes.AllFromFront() {

		builder.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF79C6")).
			Render(fmt.Sprintf("\n📁 %s", flakeName)))

		for configurationName, configuration := range flake.Configurations.AllFromFront() {

			builder.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8BE9FD")).
				Render(fmt.Sprintf("\n  📦 %s", configurationName)))

			// Process configuration logs (if any)
			if configuration != nil {
				m.renderLogsForTarget(&builder, configuration.Logs, "      ")
			}

			for machineName, machine := range configuration.Machines.AllFromFront() {
				machineHeader := fmt.Sprintf("\n    🖥️  %s", strings.TrimPrefix(machineName.String(), "ssh://"))
				builder.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFB86C")).
					Render(machineHeader) + "\n")

				m.renderLogsForTarget(&builder, machine.Logs, "      ")
			}
		}
	}

	return builder.String()
}

// renderLogsForTarget renders logs for a specific target (configuration or machine)
func (m *model) renderLogsForTarget(builder *strings.Builder, logs map[workflow_definition.WorkflowPhase]*config.Log, indent string) {
	if len(logs) == 0 {
		return
	}

	// Process all phases in order
	phases := []workflow_definition.WorkflowPhase{
		workflow_definition.PhaseStatus,
		workflow_definition.PhaseBuild,
		workflow_definition.PhaseBootstrap,
		workflow_definition.PhaseTransfer,
		workflow_definition.PhaseSecrets,
		workflow_definition.PhaseActivate,
		workflow_definition.PhaseRollback,
	}

	for _, phase := range phases {
		xpath := string(phase)

		log, exists := logs[phase]
		if !exists || log == nil {
			continue
		}

		// Calculate phase duration
		tas := log.TimeAndState.GetTimeAndState()
		var durationStr string
		var statusIcon string
		var statusColor lipgloss.Color

		if !tas.StartTime.IsZero() && !tas.EndTime.IsZero() {
			duration := tas.EndTime.Sub(tas.StartTime)
			durationStr = fmt.Sprintf("(%.2fs)", duration.Seconds())
			statusIcon = "✓"
			statusColor = "#50FA7B"
		} else if !tas.StartTime.IsZero() && !tas.Finished {
			// Live elapsed time
			elapsed := time.Since(tas.StartTime)
			durationStr = fmt.Sprintf("(%.2fs)", elapsed.Seconds())
			statusIcon = "⟳"
			statusColor = "#F1FA8C"
		} else if tas.Error != nil {
			durationStr = "(failed)"
			statusIcon = "✗"
			statusColor = "#FF5555"
		}

		// Phase header with spinner on left and duration on right
		spinnerPrefix := ""
		if !tas.StartTime.IsZero() && !tas.Finished {
			spinnerChar := m.modelView.spinners.Spinner(xpath).View()
			spinnerPrefix = spinnerChar + " "
		}

		phaseHeader := fmt.Sprintf("%s%s%s", indent, spinnerPrefix, strings.ToUpper(string(phase)))

		// Calculate padding for right-aligned duration
		if durationStr != "" {
			availableWidth := m.modelView.width - len(phaseHeader) - len(durationStr) - len(indent) - 2
			if availableWidth > 0 {
				phaseHeader += strings.Repeat(" ", availableWidth) + durationStr
			} else {
				phaseHeader += " " + durationStr
			}
		}

		builder.WriteString(lipgloss.NewStyle().
			Foreground(statusColor).
			Render(phaseHeader) + "\n")

		// Show error details if failed
		if tas.Error != nil {
			errorMsg := fmt.Sprintf("%s  %s %v", indent, statusIcon, tas.Error)
			builder.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF5555")).
				Render(errorMsg) + "\n")
		}

		// Commands and their output
		for cmdIdx, cmd := range log.Commands {
			xpath += cmd.Command

			if cmd.Command != "" {
				// Calculate command duration
				cmdTas := cmd.TimeAndState.GetTimeAndState()
				var durationStr string
				if !cmdTas.StartTime.IsZero() && !cmdTas.EndTime.IsZero() {
					duration := cmdTas.EndTime.Sub(cmdTas.StartTime)
					durationStr = fmt.Sprintf("(%.2fs)", duration.Seconds())
				} else if !cmdTas.StartTime.IsZero() && !cmdTas.Finished {
					elapsed := time.Since(cmdTas.StartTime)
					durationStr = fmt.Sprintf("(%.2fs)", elapsed.Seconds())
				}

				// Command header with spinner on left and duration on right
				cmdSpinner := ""
				if !cmdTas.StartTime.IsZero() && !cmdTas.Finished {
					spinnerChar := m.modelView.spinners.Spinner(xpath).View()
					cmdSpinner = spinnerChar + " "
				}

				cmdHeader := fmt.Sprintf("%s    %s[%d/%d] %s", indent, cmdSpinner, cmdIdx+1, len(log.Commands), cmd.Command)
				if durationStr != "" {
					// Calculate padding to right-align duration
					availableWidth := m.modelView.width - len(cmdHeader) - len(durationStr) - len(indent) - 4
					if availableWidth > 0 {
						cmdHeader += strings.Repeat(" ", availableWidth) + durationStr
					} else {
						cmdHeader += " " + durationStr
					}
				}

				builder.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("#BD93F9")).
					Render(cmdHeader) + "\n")

				// Command output
				output := strings.TrimSpace(cmd.StdCombined.String())
				if output != "" {
					lines := strings.Split(output, "\n")
					for _, line := range lines {
						if strings.TrimSpace(line) != "" {
							builder.WriteString(fmt.Sprintf("%s      %s\n", indent, line))
						}
					}
				}

				// Command error status
				if cmdTas.Error != nil {
					status := fmt.Sprintf("%s    ✗ Command failed: %v", indent, cmdTas.Error)
					builder.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color("#FF5555")).
						Render(status) + "\n")
				}
			}
		}

		builder.WriteString("\n")
	}
}
