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

		m.addChildLogs(flake.Logs, flakeTree, flake.Xpath, colors)

		for _, configuration := range flake.Configurations.SortedMap() {
			configurationNode := tree.New().
				Root(colors.Configuration.Render(fmt.Sprintf("%c %s %s", colors.IconConfiguration, configuration.Name, configuration.Message)))

			m.addChildLogs(configuration.Logs, configurationNode, configuration.Xpath, colors)

			// Add machines
			for _, machine := range configuration.Machines.SortedMap() {
				machineNode := tree.New().
					Root(colors.Machine.Render(fmt.Sprintf("%c %s %s", colors.IconMachine, machine.Name, machine.Message))).Offset(0, 4)

				m.addChildLogs(machine.Logs, machineNode, machine.Xpath, colors)

				configurationNode.Child(machineNode)
			}

			flakeTree.Child(configurationNode)
		}

		builder.WriteString("\n" + flakeTree.String())
	}

	builder.WriteString("\n\n")

	return builder.String()
}

func (m *model) addChildLogs(logs *logs.PhaseLogs, treeRoot *tree.Tree, xpath string, colors *config.ColorScheme) {
	if logs.Len() > 0 {
		phaseNodes := m.phaseNodes(xpath, logs, colors)
		for _, phaseNode := range phaseNodes {
			treeRoot.Child(phaseNode)
		}
	}
}

// phaseNodes builds individual phase nodes for direct inclusion in the tree
func (m *model) phaseNodes(xpath string, phaseLogs *logs.PhaseLogs, colors *config.ColorScheme) []*tree.Tree {
	phaseNodes := make([]*tree.Tree, 0)

	for _, entry := range phaseLogs.All() {
		phase := entry.Key
		phaseLog := entry.Value

		xpath += "-" + string(phase)

		phaseTree, phaseError := m.phaseLogs(phaseLog, colors, xpath)

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
			labelXpath := commandXpath + "-label"

			var cmdHeader string

			leftIcon := m.IconOrSpinner(commandXpath, iconOnFinished, cmdTas)
			duration := m.Duration(cmdTas)

			// Create viewport for just the command label content
			cmdLabelViewport := m.modelView.viewports.GetOrCreateLabelViewport(labelXpath, cmdLabel, 23)

			// Build the first line with left icon, first line of viewport, and duration
			entry := leftIcon + cmdLabelViewport +
				duration

			cmdHeader = colors.Command.Render(entry)
			cmdTree := tree.New().Root(cmdHeader)

			// Command output
			output := strings.TrimSpace(cmdOutput)
			if len(output) != 0 {
				outputViewport := m.modelView.viewports.GetOrCreateViewport(commandXpath+"-output", output, cmd.Pty, 19)
				cmdTree.Child(outputViewport)
			} else {
				m.modelView.viewports.RemoveIfExistsViewport(commandXpath + "-output")
			}

			// Command error status
			err := cmdTas.Error
			if err != nil {
				cmdTree.Child(colors.Error.Render(fmt.Sprintf("✗ Command failed: %s", err.Error())))
			}

			phaseTree.Child(cmdTree)
		}

		// Show error details if failed
		if phaseError != nil {
			errorMsg := colors.Error.Render(fmt.Sprintf("✗ Phase failed: %s", phaseError.Error()))
			phaseTree.Child(errorMsg)
		}

		phaseNodes = append(phaseNodes, phaseTree)
	}

	return phaseNodes
}

func (m *model) phaseLogs(phaseLog *logs.PhaseLog, colors *config.ColorScheme, xpath string) (*tree.Tree, error) {
	tas := phaseLog.TimeAndState().GetTimeAndState()

	// Phase header with spinner and right-aligned timing
	phaseLabel := strings.ToUpper(string(phaseLog.Phase()))

	phaseText := m.MostLeftAndMostRight(12,
		m.IconOrSpinner(xpath, string(colors.IconPhase), tas)+" "+phaseLabel,
		m.Duration(tas),
	)

	phaseHeader := colors.Phase.Render(phaseText)
	phaseTree := tree.New().Root(phaseHeader)

	return phaseTree, tas.Error
}

// Helpers

func (m *model) MostLeftAndMostRight(prefixLen int, left, right string) string {
	terminalWidth := m.modelView.dimensions.Width
	available := terminalWidth - prefixLen

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	originalContentWidth := leftWidth + rightWidth

	// Ensure we don't exceed available width
	if originalContentWidth > available {
		// Left side too big, truncate it
		truncatingIndicator := "... "
		truncatingIndicatorWidth := lipgloss.Width(truncatingIndicator)
		leftWidth = available - rightWidth - truncatingIndicatorWidth

		left = lipgloss.NewStyle().MaxWidth(leftWidth).Render(right) + truncatingIndicator
	} else if originalContentWidth < available {
		leftWidth = available - rightWidth
	}

	// Create layout with lipgloss
	leftBlock := lipgloss.NewStyle().Width(leftWidth).Render(left)
	rightBlock := lipgloss.NewStyle().Width(rightWidth).Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Left, leftBlock, rightBlock)
}

func (m *model) IconOrSpinner(spinnerXpath, iconOnFinished string, tas time_and_state.TimeAndStateInternal) string {
	var iconOrSpinner string

	if tas.Started && tas.Finished {
		// Icon
		iconOrSpinner = iconOnFinished
		m.modelView.spinners.RemoveIfExistsSpinner(spinnerXpath)
	} else if tas.Started && !tas.Finished {
		// Spinner
		iconOrSpinner = m.modelView.spinners.GetOrCreateSpinner(spinnerXpath).View()
	}

	final := iconOrSpinner

	return final
}

func (m *model) Duration(tas time_and_state.TimeAndStateInternal) string {
	var durationStr string

	if tas.Started && tas.Finished {
		// Finished timer
		duration := tas.EndTime.Sub(tas.StartTime)
		durationStr = fmt.Sprintf("(%.2fs)", duration.Seconds())
	} else if tas.Started && !tas.Finished {
		// Live elapsed time
		elapsed := time.Since(tas.StartTime)
		durationStr = fmt.Sprintf("(%.2fs)", elapsed.Seconds())
	}

	return durationStr
}
