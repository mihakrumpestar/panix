package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
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

	enumeratorStyle := lipgloss.NewStyle()

	// Define styles for different tree elements
	flakeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F1FA8C"))

	configStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB86C"))

	machineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8BE9FD"))

	phaseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF79C6"))

	commandStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#BD93F9"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555"))

	// Build separate trees for each flake
	for flakeName, flake := range m.state.Conf.Flakes.AllFromFront() {
		flakeTree := tree.New().
			Root(flakeStyle.Render(fmt.Sprintf("📁 %s", flakeName))).
			Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(enumeratorStyle)

		for configurationName, configuration := range flake.Configurations.AllFromFront() {
			configNode := tree.New().
				Root(configStyle.Render(fmt.Sprintf("📦 %s", configurationName)))

			// Add configuration logs directly (no "Logs" intermediate node)
			if configuration != nil && len(configuration.Logs) > 0 {
				xpath := flakeName + configurationName
				phaseNodes := m.buildPhaseNodes(xpath, configuration.Logs, phaseStyle, commandStyle, errorStyle)
				for _, phaseNode := range phaseNodes {
					configNode.Child(phaseNode)
				}
			}

			// Add machines
			for machineName, machine := range configuration.Machines.AllFromFront() {
				machineNode := tree.New().
					Root(machineStyle.Render(fmt.Sprintf("🖥️  %s", strings.TrimPrefix(machineName.String(), "ssh://")))).Offset(0, 4)

				if len(machine.Logs) > 0 {
					xpath := flakeName + configurationName + machineName.String()
					phaseNodes := m.buildPhaseNodes(xpath, machine.Logs, phaseStyle, commandStyle, errorStyle)
					for _, phaseNode := range phaseNodes {
						machineNode.Child(phaseNode)
					}
				}

				configNode.Child(machineNode)
			}

			flakeTree.Child(configNode)
		}

		builder.WriteString("\n" + flakeTree.String())
	}

	return builder.String()
}

// buildPhaseNodes builds individual phase nodes for direct inclusion in the tree
func (m *model) buildPhaseNodes(xpath string, logs map[workflow_definition.WorkflowPhase]*config.Log, phaseStyle, commandStyle, errorStyle lipgloss.Style) []*tree.Tree {
	phaseNodes := make([]*tree.Tree, 0)

	if len(logs) == 0 {
		return phaseNodes
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
		log, exists := logs[phase]
		if !exists || log == nil {
			continue
		}

		xpath += string(phase)
		tas := log.TimeAndState.GetTimeAndState()

		// Phase header with spinner and right-aligned timing
		iconOnFinished := "📋 "
		phaseLabel := strings.ToUpper(string(phase))

		phaseText := m.MostLeftAndMostRight(
			12,
			m.LeftSideIconOrSpinner(xpath, iconOnFinished, phaseLabel, tas),
			m.RightSideDuration(tas),
		)

		phaseHeader := phaseStyle.Render(phaseText)
		phaseTree := tree.New().Root(phaseHeader)

		// Commands and their output
		for cmdIdx, cmd := range log.Commands {
			if cmd.Command != "" {
				xpath += cmd.Command
				cmdTas := cmd.GetTimeAndState()

				iconOnFinished = fmt.Sprintf("%d ", cmdIdx+1)
				cmdLabel := cmd.Command

				cmdText := m.MostLeftAndMostRight(
					12,
					m.LeftSideIconOrSpinner(xpath, iconOnFinished, cmdLabel, cmdTas),
					m.RightSideDuration(tas),
				)

				cmdHeader := commandStyle.Render(cmdText)
				cmdTree := tree.New().Root(cmdHeader)

				// Command output
				output := strings.TrimSpace(cmd.StdCombined.String())
				if output != "" {
					cmdTree.Child(output)
				}

				// Command error status
				if cmdTas.Error != nil {
					cmdTree.Child(errorStyle.Render(fmt.Sprintf("✗ Command failed: %v", cmdTas.Error)))
				}

				phaseTree.Child(cmdTree)
			}
		}

		// Show error details if failed
		if tas.Error != nil {
			errorMsg := errorStyle.Render(fmt.Sprintf("✗ Phase failed: %v", tas.Error))
			phaseTree.Child(errorMsg)
		}

		phaseNodes = append(phaseNodes, phaseTree)
	}

	return phaseNodes
}

// Helpers

func (m *model) MostLeftAndMostRight(prefixLen int, left, right string) string {
	termW := m.modelView.width

	avail := termW - prefixLen
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)

	// Will have to cut left as it goes over the terminal width
	if lw+rw > avail {
		maxSafeLeftWidth := avail - rw

		fmt.Println("avail:", avail, "lw:", lw, "rw:", rw, "maxSafeLeftWidth:", maxSafeLeftWidth)

		left = left[:maxSafeLeftWidth-3] + "..."
	}

	leftBlock := lipgloss.Place(
		prefixLen+lw, 1,
		lipgloss.Left, lipgloss.Center,
		left,
	)

	rightBlock := lipgloss.Place(
		avail-lw, 1,
		lipgloss.Right, lipgloss.Center,
		right,
	)

	return lipgloss.JoinHorizontal(0, leftBlock, rightBlock)
}

func (m *model) RightSideDuration(tas config.TimeAndStateOutput) string {
	var durationStr string

	if tas.Started && tas.Finished {
		duration := tas.EndTime.Sub(tas.StartTime)
		durationStr = fmt.Sprintf("(%.2fs)", duration.Seconds())
	} else if tas.Started && !tas.Finished {
		// Live elapsed time
		elapsed := time.Since(tas.StartTime)
		durationStr = fmt.Sprintf("(%.2fs)", elapsed.Seconds())
	}

	return durationStr
}

func (m *model) LeftSideIconOrSpinner(spinnerXpath, iconOnFinished, content string, tas config.TimeAndStateOutput) string {
	var iconOrSpinner string

	if tas.Started && tas.Finished {
		iconOrSpinner = iconOnFinished
	} else if tas.Started && !tas.Finished {
		// Spinner
		iconOrSpinner = m.modelView.spinners.Spinner(spinnerXpath).View()
	}

	final := iconOrSpinner + content

	return final
}
