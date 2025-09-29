package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/mihakrumpestar/panix/internal/config"
)

// Render generates the Docker-style build log view with tree structure
func (m *model) PrintBuildLogs() string {
	var builder strings.Builder

	// Header for the log view
	builder.WriteString("\n" + config.DefaultColorScheme().HeaderTitle.Render("=== Build Logs ===\n"))

	// Build separate trees for each flake
	for _, flake := range m.workflow.State().Conf.Root.Flakes.SortedMap(false, false) {
		flakeTree := tree.New().
			Root(config.DefaultColorScheme().Flake.Render(fmt.Sprintf("%c %s %s", config.DefaultColorScheme().IconFlake, flake.Name, flake.Message))).
			Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(config.DefaultColorScheme().TreeEnumerator)

		if flake != nil && flake.Logs.Len() > 0 {
			xpath := flake.Name
			phaseNodes := m.phaseNodes(xpath, flake.Logs)
			for _, phaseNode := range phaseNodes {
				flakeTree.Child(phaseNode)
			}
		}

		for _, configuration := range flake.Configurations.SortedMap(false, false) {
			configNode := tree.New().
				Root(config.DefaultColorScheme().Configuration.Render(fmt.Sprintf("%c %s %s", config.DefaultColorScheme().IconConfiguration, configuration.Name, configuration.Message)))

			// Add configuration logs directly (no "Logs" intermediate node)
			if configuration.Logs.Len() > 0 {
				xpath := flake.Name + configuration.Name
				phaseNodes := m.phaseNodes(xpath, configuration.Logs)
				for _, phaseNode := range phaseNodes {
					configNode.Child(phaseNode)
				}
			}

			// Add machines
			for _, machine := range configuration.Machines.SortedMap(false, false) {
				machineNode := tree.New().
					Root(config.DefaultColorScheme().Machine.Render(fmt.Sprintf("%c %s %s", config.DefaultColorScheme().IconMachine, machine.Name, machine.Message))).Offset(0, 4)

				if machine.Logs.Len() > 0 {
					xpath := flake.Name + configuration.Name + machine.Name
					phaseNodes := m.phaseNodes(xpath, machine.Logs)
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
func (m *model) phaseNodes(xpath string, logs *config.PhaseLogs) []*tree.Tree {
	phaseNodes := make([]*tree.Tree, 0)

	if logs.Len() == 0 {
		return phaseNodes
	}

	for phase, log := range logs.All() {
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

		phaseHeader := config.DefaultColorScheme().Phase.Render(phaseText)
		phaseTree := tree.New().Root(phaseHeader)

		// Commands and their output
		for cmdIdx, cmd := range log.Commands {
			if cmd.Command != "" {
				commandXpath := xpath + cmd.Command
				cmdTas := cmd.TimeAndState.GetTimeAndState()

				iconOnFinished = fmt.Sprintf("%d ", cmdIdx+1)
				cmdLabel := cmd.Command

				cmdText := m.MostLeftAndMostRight(
					12,
					m.LeftSideIconOrSpinner(commandXpath, iconOnFinished, cmdLabel, cmdTas),
					m.RightSideDuration(cmdTas),
				)

				cmdHeader := config.DefaultColorScheme().Command.Render(cmdText)
				cmdTree := tree.New().Root(cmdHeader)

				// Command output
				output := strings.TrimSpace(cmd.String())
				if len(output) != 0 {
					vpr := m.modelView.viewports.GetOrCreateViewport(commandXpath, output, cmd.Pty, 4)
					cmdTree.Child(vpr)
				} else {
					m.modelView.viewports.RemoveIfExistsViewport(commandXpath)
				}

				// Command error status
				if cmdTas.Error != nil {
					cmdTree.Child(config.DefaultColorScheme().Error.Render(fmt.Sprintf("✗ Command failed: %v", cmdTas.Error)))
				}

				phaseTree.Child(cmdTree)
			}
		}

		// Show error details if failed
		if tas.Error != nil {
			errorMsg := config.DefaultColorScheme().Error.Render(fmt.Sprintf("✗ Phase failed: %v", tas.Error))
			phaseTree.Child(errorMsg)
		}

		phaseNodes = append(phaseNodes, phaseTree)
	}

	return phaseNodes
}

// Helpers

func (m *model) MostLeftAndMostRight(prefixLen int, left, right string) string {
	termW := m.modelView.dimensions.width
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

// lastNLines returns the last n lines of s.
// If s has fewer than n lines, it returns s unchanged.
func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
