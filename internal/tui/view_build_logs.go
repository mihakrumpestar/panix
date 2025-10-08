package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// Render generates the Docker-style build log view with tree structure
func (m *model) ViewBuildLogs() string {
	var builder strings.Builder

	colors := m.workflow.State().Conf.Tui.ColorScheme

	// Header for the log view
	builder.WriteString(colors.HeaderTitle.Render("=== Build Logs ===\n"))

	// Build separate trees for each flake
	for _, flake := range m.workflow.State().Conf.Root.Flakes.SortedMap() {
		flakeTree := tree.New().
			Root(colors.Flake.Render(fmt.Sprintf("%c %s %s", colors.IconFlake, flake.Attributes.Name, flake.Attributes.Message))).
			Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(colors.TreeEnumerator)

		if flake.Logs.Len() > 0 {
			xpath := flake.Name
			phaseNodes := m.phaseNodes(xpath, flake.Logs, colors)
			for _, phaseNode := range phaseNodes {
				flakeTree.Child(phaseNode)
			}
		}

		for _, configuration := range flake.Configurations.SortedMap() {
			configNode := tree.New().
				Root(colors.Configuration.Render(fmt.Sprintf("%c %s %s", colors.IconConfiguration, configuration.Name, configuration.Message)))

			// Add configuration logs directly (no "Logs" intermediate node)
			if configuration.Logs.Len() > 0 {
				xpath := flake.Name + configuration.Name
				phaseNodes := m.phaseNodes(xpath, configuration.Logs, colors)
				for _, phaseNode := range phaseNodes {
					configNode.Child(phaseNode)
				}
			}

			// Add machines
			for _, machine := range configuration.Machines.SortedMap() {
				machineNode := tree.New().
					Root(colors.Machine.Render(fmt.Sprintf("%c %s %s", colors.IconMachine, machine.Name, machine.Message))).Offset(0, 4)

				if machine.Logs.Len() > 0 {
					xpath := flake.Name + configuration.Name + machine.Name
					phaseNodes := m.phaseNodes(xpath, machine.Logs, colors)
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

	builder.WriteString("\n\n")

	return builder.String()
}

// phaseNodes builds individual phase nodes for direct inclusion in the tree
func (m *model) phaseNodes(xpath string, phaseLogs *logs.PhaseLogs, colors *config.ColorScheme) []*tree.Tree {
	phaseNodes := make([]*tree.Tree, 0)

	if phaseLogs.Len() == 0 {
		return phaseNodes
	}

	for _, entry := range phaseLogs.All() {
		phase := entry.Key
		phaseLog := entry.Value

		xpath += string(phase)
		tas := phaseLog.TimeAndState().GetTimeAndState()

		// Phase header with spinner and right-aligned timing
		phaseLabel := strings.ToUpper(string(phase))

		phaseText := m.MostLeftAndMostRight(
			12,
			m.LeftSideIconOrSpinner(xpath, string(colors.IconPhase), phaseLabel, tas),
			m.RightSideDuration(tas),
		)

		phaseHeader := colors.Phase.Render(phaseText)
		phaseTree := tree.New().Root(phaseHeader)

		// Commands and their output
		for cmdIdx, cmd := range phaseLog.CommandLogs() {
			cmdLabel := cmd.Command.Load()
			cmdOutput := cmd.String()
			cmdTas := cmd.TimeAndState.GetTimeAndState()

			if !m.workflow.State().Conf.Tui.ShowAllBuildLogs && slices.Contains([]phases.Phase{phases.Status, phases.Secrets}, phase) && cmdTas.Error == nil {
				if cmdIdx == len(phaseLog.CommandLogs())-1 {
					cmdLabel = "(hidden)"
					cmdOutput = ""
				} else {
					continue
				}
			}

			iconOnFinished := fmt.Sprintf("%d ", cmdIdx+1)
			commandXpath := xpath + cmdLabel

			cmdText := m.MostLeftAndMostRight(
				12,
				m.LeftSideIconOrSpinner(commandXpath, iconOnFinished, cmdLabel, cmdTas),
				m.RightSideDuration(cmdTas),
			)

			cmdHeader := colors.Command.Render(cmdText)
			cmdTree := tree.New().Root(cmdHeader)

			// Command output
			output := strings.TrimSpace(cmdOutput)
			if len(output) != 0 {
				vpr := m.modelView.viewports.GetOrCreateViewport(commandXpath, output, cmd.Pty, 4)
				cmdTree.Child(vpr)
			} else {
				m.modelView.viewports.RemoveIfExistsViewport(commandXpath)
			}

			// Command error status
			err := cmdTas.Error
			if err != nil {
				cmdTree.Child(colors.Error.Render(fmt.Sprintf("✗ Command failed: %s", err.Error())))
			}

			phaseTree.Child(cmdTree)
		}

		// Show error details if failed
		err := tas.Error
		if err != nil {
			errorMsg := colors.Error.Render(fmt.Sprintf("✗ Phase failed: %s", err.Error()))
			phaseTree.Child(errorMsg)
		}

		phaseNodes = append(phaseNodes, phaseTree)
	}

	return phaseNodes
}

// Helpers

func (m *model) MostLeftAndMostRight(prefixLen int, left, right string) string {
	termW := m.modelView.dimensions.Width
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

func (m *model) RightSideDuration(tas time_and_state.TimeAndStateInternal) string {
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

func (m *model) LeftSideIconOrSpinner(spinnerXpath, iconOnFinished, content string, tas time_and_state.TimeAndStateInternal) string {
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
