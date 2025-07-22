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

	// Use color scheme from model
	colors := m.modelView.colors

	// Header for the log view
	builder.WriteString("\n" + colors.HeaderTitle.Render("=== Build Logs ===\n"))

	enumeratorStyle := colors.TreeEnumerator

	// Use color scheme styles for different tree elements
	flakeStyle := colors.Flake
	configStyle := colors.Configuration
	machineStyle := colors.Machine
	phaseStyle := colors.Phase
	commandStyle := colors.Command
	errorStyle := colors.Error

	// Build separate trees for each flake
	for flakeName, flake := range m.workflow.State().Conf.Flakes.AllFromFront() {
		flakeTree := tree.New().
			Root(flakeStyle.Render(fmt.Sprintf("%c %s", colors.IconFlake, flakeName))).
			Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(enumeratorStyle)

		for configurationName, configuration := range flake.Configurations.AllFromFront() {
			configNode := tree.New().
				Root(configStyle.Render(fmt.Sprintf("%c %s", colors.IconConfiguration, configurationName)))

			// Add configuration logs directly (no "Logs" intermediate node)
			if configuration != nil && len(configuration.Logs) > 0 {
				xpath := flakeName + configurationName
				phaseNodes := m.phaseNodes(xpath, configuration.Logs, phaseStyle, commandStyle, errorStyle)
				for _, phaseNode := range phaseNodes {
					configNode.Child(phaseNode)
				}
			}

			// Add machines
			for machineName, machine := range configuration.Machines.AllFromFront() {
				machineNode := tree.New().
					Root(machineStyle.Render(fmt.Sprintf("%c %s", colors.IconMachine, strings.TrimPrefix(machineName.String(), "ssh://")))).Offset(0, 4)

				if len(machine.Logs) > 0 {
					xpath := flakeName + configurationName + machineName.String()
					phaseNodes := m.phaseNodes(xpath, machine.Logs, phaseStyle, commandStyle, errorStyle)
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

// phaseNodes builds individual phase nodes for direct inclusion in the tree
func (m *model) phaseNodes(xpath string, logs map[workflow_definition.WorkflowPhase]*config.Log, phaseStyle, commandStyle, errorStyle lipgloss.Style) []*tree.Tree {
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
		if !exists || log == nil || len(log.Commands) == 0 {
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
					m.RightSideDuration(cmdTas),
				)

				cmdHeader := commandStyle.Render(cmdText)
				cmdTree := tree.New().Root(cmdHeader)

				// Command output
				output := strings.TrimSpace(cmd.StdInOutErr.String())
				if output != "" {
					truncate := lastLines(output, 8)
					cmdTree.Child(truncate)
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

	// Ensure we don't exceed available width
	if rw >= avail {
		// Right side too big, truncate it
		right = lipgloss.NewStyle().MaxWidth(avail).Render(right)
		return right
	}

	// Calculate max width for left side
	maxLeft := avail - rw
	if lw > maxLeft {
		// Truncate left side
		truncatingIndicator := "...  "
		left = lipgloss.NewStyle().MaxWidth(maxLeft-lipgloss.Width(truncatingIndicator)).Render(left) + truncatingIndicator
	}

	// Create layout with lipgloss
	leftBlock := lipgloss.NewStyle().Width(avail - rw).Render(left)
	rightBlock := lipgloss.NewStyle().Width(rw).Align(lipgloss.Right).Render(right)

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
		m.modelView.spinners.RemoveIfExistsSpinner(spinnerXpath)
	} else if tas.Started && !tas.Finished {
		// Spinner
		iconOrSpinner = m.modelView.spinners.GetOrCreateSpinner(spinnerXpath).View()
	}

	final := iconOrSpinner + content

	return final
}

// lastLines returns the last n lines of s.
// If s has fewer than n lines, it returns s unchanged.
func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
