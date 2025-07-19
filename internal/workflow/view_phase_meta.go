package workflow

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

// BuildLogView represents a Docker-style multi-step build log view
type BuildLogView struct {
	width int
}

// NewBuildLogView creates a new build log view with the specified width
func NewBuildLogView(width int) *BuildLogView {
	return &BuildLogView{width: width}
}

// Render generates the Docker-style build log view with tree structure
func (v *BuildLogView) Render(state *WorkflowState, spinnerFrame int) string {
	if state == nil {
		return "No workflow state available"
	}

	var builder strings.Builder

	// Header for the log view
	builder.WriteString("\n" + lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ADD8")).
		Render("=== Build Logs ===\n"))

	// Collect all entries in order
	type machineEntry struct {
		flakeName         string
		configurationName string
		configuration     *config.Configuration
		machineName       url.URL
		machine           *config.Machine
	}

	var entries []machineEntry
	state.expandFlakeConfigurationMachine(func(i int, flakeName, configurationName string, configuration *config.Configuration, machineName url.URL, machine *config.Machine) {
		entries = append(entries, machineEntry{
			flakeName:         flakeName,
			configurationName: configurationName,
			configuration:     configuration,
			machineName:       machineName,
			machine:           machine,
		})
	})

	// Process entries in order, grouping by flake and configuration
	currentFlake := ""
	currentConfig := ""

	for _, entry := range entries {
		// Flake header (only once per flake)
		if entry.flakeName != currentFlake {
			builder.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF79C6")).
				Render(fmt.Sprintf("\n📁 %s", entry.flakeName)))
			currentFlake = entry.flakeName
			currentConfig = "" // Reset config when flake changes
		}

		// Configuration header (only once per configuration)
		configKey := entry.flakeName + "/" + entry.configurationName
		if configKey != currentConfig {
			builder.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8BE9FD")).
				Render(fmt.Sprintf("\n  📦 %s", entry.configurationName)))
			currentConfig = configKey
		}

		// Machine header
		machineHeader := fmt.Sprintf("\n    🖥️  %s", strings.TrimPrefix(entry.machineName.String(), "ssh://"))
		builder.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")).
			Render(machineHeader) + "\n")

		// Process configuration logs (if any)
		if entry.configuration != nil {
			v.renderLogsForTarget(&builder, entry.configuration.Logs, "      ", spinnerFrame)
		}

		// Process machine logs
		v.renderLogsForTarget(&builder, entry.machine.Logs, "      ", spinnerFrame)
	}

	return builder.String()
}

// renderLogsForTarget renders logs for a specific target (configuration or machine)
func (v *BuildLogView) renderLogsForTarget(builder *strings.Builder, logs map[workflow_definition.WorkflowPhase]*config.Log, indent string, spinnerFrame int) {
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

	// Spinner characters for animation
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	for _, phase := range phases {
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
			spinnerChar := spinners[spinnerFrame%len(spinners)]
			spinnerPrefix = spinnerChar + " "
		}

		phaseHeader := fmt.Sprintf("%s%s%s", indent, spinnerPrefix, strings.ToUpper(string(phase)))

		// Calculate padding for right-aligned duration
		if durationStr != "" {
			availableWidth := v.width - len(phaseHeader) - len(durationStr) - len(indent) - 2
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
					spinnerChar := spinners[spinnerFrame%len(spinners)]
					cmdSpinner = spinnerChar + " "
				}

				cmdHeader := fmt.Sprintf("%s    %s[%d/%d] %s", indent, cmdSpinner, cmdIdx+1, len(log.Commands), cmd.Command)
				if durationStr != "" {
					// Calculate padding to right-align duration
					availableWidth := v.width - len(cmdHeader) - len(durationStr) - len(indent) - 4
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

// Helper method to get configuration from state
func (state *WorkflowState) getConfiguration(flakeName, configName string) *config.Configuration {
	var foundConfig *config.Configuration
	state.expandFlakeConfigurationMachine(func(i int, fName, cName string, configuration *config.Configuration, machineName url.URL, machine *config.Machine) {
		if fName == flakeName && cName == configName && foundConfig == nil {
			foundConfig = configuration
		}
	})
	return foundConfig
}

// PrintBuildLogs returns the Docker-style build logs as a string
func (state *WorkflowState) PrintBuildLogs(width int, spinnerFrame int) string {
	view := NewBuildLogView(width)
	return view.Render(state, spinnerFrame)
}

// GetCombinedView returns both the status table and build logs combined
func (state *WorkflowState) GetCombinedView(width int, spinnerFrame int) (string, error) {
	var builder strings.Builder

	// Get status table
	statusTable, err := state.PrintStatusPhaseMachineTable(width, spinnerFrame)
	if err != nil {
		return "", fmt.Errorf("failed to generate status table: %w", err)
	}

	if statusTable != nil {
		builder.WriteString(statusTable.String())
	}

	// Add build logs
	buildLogs := state.PrintBuildLogs(width, spinnerFrame)
	builder.WriteString(buildLogs)

	return builder.String(), nil
}
