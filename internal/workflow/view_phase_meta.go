package workflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/executioner"
)

// PrintPhaseMetaTable displays all meta information from Executioner output in a table format
func (meta *Metadatas) PrintPhaseMetaTable() (*table.Table, error) {
	if config.C.Global.DryRun {
		if config.C.Global.Verbose {
			fmt.Println("No phase meta table when dry-run option is enabled")
		}
		return nil, nil
	}

	if meta == nil {
		return nil, fmt.Errorf("meta is nil")
	}

	// Create a comprehensive table showing all execution metadata
	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("FLAKE", "CONFIGURATION", "MACHINE", "PHASE", "STATUS", "DURATION", "COMMANDS", "ERRORS")

	// If we have status phase meta, show machine-level execution details
	if meta.StatusPhaseMeta != nil && meta.StatusPhaseMeta.MachineStatuses != nil {
		for _, sm := range meta.StatusPhaseMeta.MachineStatuses {
			if sm.BaseMeta != nil {
				addBaseMetaToTable(t, sm.BaseMeta, "STATUS")
			}
		}
	}

	return t, nil
}

// PrintDetailedPhaseMeta displays detailed execution metadata including command outputs
func (meta *Metadatas) PrintDetailedPhaseMeta() (string, error) {
	if config.C.Global.DryRun {
		if config.C.Global.Verbose {
			return "No detailed phase meta when dry-run option is enabled", nil
		}
		return "", nil
	}

	if meta == nil {
		return "", fmt.Errorf("meta is nil")
	}

	var sb strings.Builder

	// Header styling
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF00")).
		Background(lipgloss.Color("#000080"))

	// Section styling
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFF00"))

	// Error styling
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	sb.WriteString(headerStyle.Render("=== EXECUTION METADATA SUMMARY ===\n\n"))

	// Global metadata
	if meta.Error != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("Global Error: %v\n\n", meta.Error)))
	}

	// Status phase metadata
	if meta.StatusPhaseMeta != nil {
		sb.WriteString(sectionStyle.Render("STATUS PHASE METADATA:\n"))

		if meta.StatusPhaseMeta.MetadatasBase != nil && meta.StatusPhaseMeta.MetadatasBase.Error != nil {
			sb.WriteString(errorStyle.Render(fmt.Sprintf("  Phase Error: %v\n", meta.StatusPhaseMeta.MetadatasBase.Error)))
		}

		// Safely access MachineStatuses
		machineStatuses := meta.StatusPhaseMeta.MachineStatuses
		if machineStatuses == nil {
			sb.WriteString("  No machine statuses available\n")
			return sb.String(), nil
		}

		sb.WriteString(fmt.Sprintf("  Machines Checked: %d\n\n", len(machineStatuses)))

		for i := 0; i < len(machineStatuses); i++ {
			sm := machineStatuses[i]
			if sm == nil {
				sb.WriteString(fmt.Sprintf("  Machine %d: <nil>\n", i+1))
				continue
			}

			sb.WriteString(fmt.Sprintf("  Machine %d:\n", i+1))

			// Safely access BaseMeta
			if sm.BaseMeta != nil {
				sb.WriteString(formatBaseMetaDetailed(sm.BaseMeta, "    "))
			} else {
				sb.WriteString("    <BaseMeta is nil>\n")
			}

			// Machine-specific status info with safe defaults
			statusIcon := sm.getStatusIcon()
			statusText := sm.getStatusText()
			sb.WriteString(fmt.Sprintf("    Status: %s %s\n", statusIcon, statusText))
			sb.WriteString(fmt.Sprintf("    Current Generation: %s\n", safeString(sm.CurrentGeneration)))
			sb.WriteString(fmt.Sprintf("    Last Deploy: %s\n", safeString(sm.LastDeployTime)))
			sb.WriteString(fmt.Sprintf("    Reachable: %t\n", sm.Reachable))
			sb.WriteString(fmt.Sprintf("    SSH Connectable: %t\n", sm.SSHConnectable))
			sb.WriteString(fmt.Sprintf("    Bootstrapped: %t\n", sm.Bootstrapped))
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("  No status phase metadata available\n")
	}

	return sb.String(), nil
}

// formatBaseMetaDetailed formats detailed information from BaseMeta
func formatBaseMetaDetailed(bm *executioner.BaseMeta, indent string) string {
	var sb strings.Builder

	if bm == nil {
		return fmt.Sprintf("%s<BaseMeta is nil>\n", indent)
	}

	// Basic info
	sb.WriteString(fmt.Sprintf("%sFlake: %s\n", indent, bm.FlakeName))
	sb.WriteString(fmt.Sprintf("%sConfiguration: %s\n", indent, bm.ConfigurationName))
	sb.WriteString(fmt.Sprintf("%sMachine: %s\n", indent, bm.MachineName.String()))

	// Timing info
	if !bm.StartTime.IsZero() {
		sb.WriteString(fmt.Sprintf("%sStart Time: %s\n", indent, bm.StartTime.Format(time.RFC3339)))
	}
	if !bm.EndTime.IsZero() {
		sb.WriteString(fmt.Sprintf("%sEnd Time: %s\n", indent, bm.EndTime.Format(time.RFC3339)))
		duration := bm.EndTime.Sub(bm.StartTime)
		sb.WriteString(fmt.Sprintf("%sDuration: %v\n", indent, duration))
	} else {
		sb.WriteString(fmt.Sprintf("%sStatus: %s\n", indent, spinner.New().View()))
	}

	// Error info
	if bm.Error != nil {
		sb.WriteString(fmt.Sprintf("%sError: %v\n", indent, bm.Error))
	}

	// Command outputs
	if len(bm.CommandOutputs) > 0 {
		sb.WriteString(fmt.Sprintf("%sCommands Executed: %d\n", indent, len(bm.CommandOutputs)))

		for i, cmd := range bm.CommandOutputs {
			sb.WriteString(fmt.Sprintf("%s  Command %d: %s\n", indent, i+1, cmd.Command))

			if !cmd.StartTime.IsZero() && !cmd.EndTime.IsZero() {
				duration := cmd.EndTime.Sub(cmd.StartTime)
				sb.WriteString(fmt.Sprintf("%s    Duration: %v\n", indent, duration))
			}

			if cmd.Stdout.Len() > 0 {
				stdout := strings.TrimSpace(cmd.Stdout.String())
				if stdout != "" {
					sb.WriteString(fmt.Sprintf("%s    Stdout: %s\n", indent, truncateString(stdout, 100)))
				}
			}

			if cmd.Stderr.Len() > 0 {
				stderr := strings.TrimSpace(cmd.Stderr.String())
				if stderr != "" {
					sb.WriteString(fmt.Sprintf("%s    Stderr: %s\n", indent, truncateString(stderr, 100)))
				}
			}

			if cmd.Error != nil {
				sb.WriteString(fmt.Sprintf("%s    Error: %v\n", indent, cmd.Error))
			}
		}
	}

	return sb.String()
}

// addBaseMetaToTable adds BaseMeta information to a table
func addBaseMetaToTable(t *table.Table, bm *executioner.BaseMeta, phase string) {
	if bm == nil {
		return
	}

	status := "RUNNING"
	if !bm.EndTime.IsZero() {
		if bm.Error != nil {
			status = "FAILED"
		} else {
			status = "SUCCESS"
		}
	}

	duration := ""
	if !bm.StartTime.IsZero() && !bm.EndTime.IsZero() {
		duration = bm.EndTime.Sub(bm.StartTime).String()
	} else if !bm.StartTime.IsZero() {
		duration = time.Since(bm.StartTime).String() + " (running)"
	}

	commandCount := fmt.Sprintf("%d", len(bm.CommandOutputs))

	errorMsg := ""
	if bm.Error != nil {
		errorMsg = bm.Error.Error()
	}

	machineName := ""
	machineName = bm.MachineName.String()

	t.Row(
		bm.FlakeName,
		bm.ConfigurationName,
		machineName,
		phase,
		status,
		duration,
		commandCount,
		truncateString(errorMsg, 50),
	)
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// PrintCommandOutputs displays detailed command outputs for a specific BaseMeta
func PrintCommandOutputs(bm *executioner.BaseMeta) string {
	if bm == nil {
		return "<BaseMeta is nil>"
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FFFF"))

	machineName := ""
	machineName = bm.MachineName.String()

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Command Outputs for %s/%s/%s\n",
		bm.FlakeName, bm.ConfigurationName, machineName)))
	sb.WriteString(strings.Repeat("=", 80) + "\n\n")

	for i, cmd := range bm.CommandOutputs {
		sb.WriteString(fmt.Sprintf("Command %d: %s\n", i+1, cmd.Command))
		sb.WriteString(fmt.Sprintf("Start: %s\n", cmd.StartTime.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("End: %s\n", cmd.EndTime.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("Duration: %v\n", cmd.EndTime.Sub(cmd.StartTime)))

		if cmd.Stdout.Len() > 0 {
			sb.WriteString("\n--- STDOUT ---\n")
			sb.WriteString(cmd.Stdout.String())
		}

		if cmd.Stderr.Len() > 0 {
			sb.WriteString("\n--- STDERR ---\n")
			sb.WriteString(cmd.Stderr.String())
		}

		if cmd.Error != nil {
			sb.WriteString(fmt.Sprintf("\n--- ERROR ---\n%v\n", cmd.Error))
		}

		sb.WriteString("\n" + strings.Repeat("-", 80) + "\n\n")
	}

	return sb.String()
}

// GetPhaseSummary returns a summary of all phases across all machines
func (meta *Metadatas) GetPhaseSummary() map[string]PhaseSummary {
	summary := make(map[string]PhaseSummary)

	if meta == nil {
		return summary
	}

	// Process status phase
	if meta.StatusPhaseMeta != nil && meta.StatusPhaseMeta.MachineStatuses != nil {
		for _, sm := range meta.StatusPhaseMeta.MachineStatuses {
			if sm.BaseMeta != nil {
				machineName := sm.BaseMeta.MachineName.String()

				key := fmt.Sprintf("%s/%s/%s",
					sm.BaseMeta.FlakeName,
					sm.BaseMeta.ConfigurationName,
					machineName)

				if _, exists := summary[key]; !exists {
					summary[key] = PhaseSummary{
						FlakeName:     sm.BaseMeta.FlakeName,
						Configuration: sm.BaseMeta.ConfigurationName,
						MachineName:   machineName,
						Phases:        make(map[string]PhaseInfo),
					}
				}

				summary[key].Phases["STATUS"] = PhaseInfo{
					StartTime:    sm.BaseMeta.StartTime,
					EndTime:      sm.BaseMeta.EndTime,
					Error:        sm.BaseMeta.Error,
					CommandCount: len(sm.BaseMeta.CommandOutputs),
				}
			}
		}
	}

	return summary
}

// safeString returns a safe string representation, handling empty strings
func safeString(s string) string {
	if s == "" {
		return "<empty>"
	}
	return s
}

// GetAllBaseMetas returns all BaseMeta objects from the metadata
func (meta *Metadatas) GetAllBaseMetas() []*executioner.BaseMeta {
	var metas []*executioner.BaseMeta

	if meta == nil {
		return metas
	}

	// Collect from status phase
	if meta.StatusPhaseMeta != nil && meta.StatusPhaseMeta.MachineStatuses != nil {
		for _, sm := range meta.StatusPhaseMeta.MachineStatuses {
			if sm != nil && sm.BaseMeta != nil {
				metas = append(metas, sm.BaseMeta)
			}
		}
	}

	return metas
}

// PhaseSummary contains summary information for a machine across all phases
type PhaseSummary struct {
	FlakeName     string
	Configuration string
	MachineName   string
	Phases        map[string]PhaseInfo
}

// PhaseInfo contains information about a specific phase
type PhaseInfo struct {
	StartTime    time.Time
	EndTime      time.Time
	Error        error
	CommandCount int
}
