package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type BuildLogHierarchy int

const (
	PhaseFirstMachineSecond BuildLogHierarchy = iota
	MachineFirstPhaseSecond
	PhaseOnly

	treeInitIndent int = 0
	treeStep       int = 3 // Real is 3
)

// Render generates the Docker-style build log view with tree structure
func (m *model) ViewBuildLogs() string {
	var builder strings.Builder

	colors := m.workflow.State().Conf.Tui.ColorScheme

	// Header for the log view
	builder.WriteString(colors.HeaderTitle.Render("=== Build Logs ===\n"))

	prefixLen := treeInitIndent

	// Build separate trees for each flake
	for _, flake := range m.workflow.State().Conf.Root.Flakes.SortedMap() {
		title := m.MostLeftAndMostRight(prefixLen,
			nodeTitle(colors.Flake, &flake.Attributes),
			m.DurationDas(colors.Flake, m.workflow.State().Conf.TargetsLogs.Get(flake.Xpath).CalculateDurationAndError()),
		)

		flakeNode := tree.New().
			Root(title).
			Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(colors.TreeEnumerator)

			// Add configurations
		prefixLen += treeStep
		for _, configuration := range flake.Configurations.SortedMap() {
			title = m.MostLeftAndMostRight(prefixLen,
				nodeTitle(colors.Configuration, &configuration.Attributes),
				m.DurationDas(colors.Configuration, m.workflow.State().Conf.TargetsLogs.Get(configuration.Xpath).CalculateDurationAndError()),
			)

			configurationNode := tree.New().Root(title)

			m.forMachines(prefixLen, configurationNode, configuration, func(prefixLen int, machine *config.Machine, machineNode *tree.Tree) {
				m.addChildLogs(prefixLen, &machine.Attributes, machineNode, colors, PhaseFirstMachineSecond, phases.Inspect)
			})

			m.addChildLogs(prefixLen, &configuration.Machines.First().Attributes, configurationNode, colors, PhaseOnly, phases.Build)

			m.forMachines(prefixLen, configurationNode, configuration, func(prefixLen int, machine *config.Machine, machineNode *tree.Tree) {
				m.addChildLogs(prefixLen, &machine.Attributes, machineNode, colors, MachineFirstPhaseSecond, phases.PhasesInOrder()[2:]...)
			})

			flakeNode.Child(configurationNode)
		}

		builder.WriteString("\n" + flakeNode.String())
	}

	builder.WriteString("\n\n")

	return builder.String()
}

func (m *model) forMachines(prefixLen int, configurationNode *tree.Tree, configuration *config.Configuration, f func(prefixLen int, machine *config.Machine, machineNode *tree.Tree)) {
	colors := m.workflow.State().Conf.Tui.ColorScheme

	prefixLen += treeStep
	for _, machine := range configuration.Machines.SortedMap() {
		title := m.MostLeftAndMostRight(6,
			nodeTitle(colors.Machine, &machine.Attributes),
			m.DurationDas(colors.Machine, m.workflow.State().Conf.TargetsLogs.Get(configuration.Xpath).CalculateDurationAndError()),
		)

		machineNode := tree.New().Root(title)

		f(prefixLen, machine, machineNode)

		if machineNode.Children().Length() != 0 {
			configurationNode.Child(machineNode)
		}
	}
}

func (m *model) addChildLogs(prefixLen int, attr *config_attributes.Attributes, treeRoot *tree.Tree, colors *config.ColorScheme, hierarchy BuildLogHierarchy, limitToPhases ...phases.Phase) {
	logs := m.workflow.State().Conf.TargetsLogs.GetLogs(attr.Xpath)

	if logs.Len() > 0 {
		phaseNodes := m.phaseNodes(prefixLen, attr.Xpath, logs, colors, hierarchy, limitToPhases...)
		for _, phaseNode := range phaseNodes {
			treeRoot.Child(phaseNode) // Passing "phaseNodes" directly does not produce the same result as adding them seperately
		}
	}
}

// phaseNodes builds individual phase nodes for direct inclusion in the tree
func (m *model) phaseNodes(prefixLen int, xpath config_attributes.Xpath, phaseLogs *logs_phase.PhaseLogs, colors *config.ColorScheme, hierarchy BuildLogHierarchy, limitToPhases ...phases.Phase) []*tree.Tree {
	phaseNodes := make([]*tree.Tree, 0)

	prefixLen += treeStep

	for _, entry := range phaseLogs.All() {
		phase := entry.Key
		phaseLog := entry.Value

		if !slices.Contains(limitToPhases, phase) {
			continue
		}

		phaseXpath := xpath.NewXpathWithAppend(string(phase))

		phaseTree, _ := m.phaseLogs(prefixLen, phaseLog, colors, phaseXpath)
		prefixLen += treeStep * 4

		// Commands and their output
		for cmdIdx, cmd := range phaseLog.CommandLogs() {
			cmdLabel := ""
			if false {
				cmdLabel = cmd.Command.Load()
			} else {
				cmdLabel = cmd.Description
			}

			cmdOutput := cmd.String()
			cmdTas := cmd.TimeAndState

			if !m.workflow.State().Conf.Tui.ShowAllBuildLogs && slices.Contains([]phases.Phase{phases.Inspect, phases.Secrets}, phase) && cmdTas.GetEndError() == nil {
				if cmdIdx == len(phaseLog.CommandLogs())-1 {
					cmdLabel = "(hidden)"
					cmdOutput = ""
				} else {
					continue
				}
			}

			iconOnFinished := fmt.Sprintf("%d ", cmdIdx+1)
			commandXpath := phaseXpath.NewXpathWithAppend(cmdLabel)
			labelXpath := commandXpath.NewXpathWithAppend("label")

			var cmdHeader string

			leftIcon := m.IconOrSpinner(commandXpath, iconOnFinished, cmdTas)
			duration := m.DurationTas(colors.Command, cmdTas)

			// Create viewport for just the command label content
			cmdLabelViewport := m.modelView.viewports.GetOrCreateLabelViewport(labelXpath, cmdLabel, prefixLen)

			// Build the first line with left icon, first line of viewport, and duration
			entry := leftIcon + cmdLabelViewport + duration

			cmdHeader = colors.Command.Color.Render(entry)
			cmdTree := tree.New().Root(cmdHeader)

			// Command output
			viewportXpath := commandXpath.NewXpathWithAppend("output")
			output := strings.TrimSpace(cmdOutput)
			if len(output) != 0 {
				outputViewport := m.modelView.viewports.GetOrCreateViewport(viewportXpath, output, cmd.Pty, prefixLen)
				cmdTree.Child(outputViewport)
			} else {
				m.modelView.viewports.RemoveIfExistsViewport(viewportXpath)
			}

			// Command error status
			err := cmdTas.GetEndError()
			if err != nil {
				cmdTree.Child(colors.Error.Color.Render(fmt.Sprintf("✗ Command failed: %s", err.Error())))
			}

			phaseTree.Child(cmdTree)
		}

		phaseNodes = append(phaseNodes, phaseTree)
	}

	return phaseNodes
}

func (m *model) phaseLogs(prefixLen int, phaseLog *logs_phase.PhaseLog, colors *config.ColorScheme, xpath config_attributes.Xpath) (*tree.Tree, error) {
	tas := phaseLog.TimeAndState()

	// Phase header with spinner and right-aligned timing
	phaseLabel := strings.ToUpper(string(phaseLog.Phase()))

	phaseText := m.MostLeftAndMostRight(prefixLen,
		m.IconOrSpinner(xpath, string(colors.Phase.Icon), tas)+" "+phaseLabel,
		m.DurationTas(colors.Phase, tas),
	)

	phaseHeader := colors.Phase.Color.Render(phaseText)
	phaseTree := tree.New().Root(phaseHeader)

	return phaseTree, tas.GetEndError()
}

// Helpers

func (m *model) MostLeftAndMostRight(prefixLen int, left, right string) string {
	availableWidth := m.modelView.dimensions.Width - prefixLen

	contentRightWidth := lipgloss.Width(right)
	contentLeftWidth := max(availableWidth-contentRightWidth, lipgloss.Width(left))

	// Create layout with lipgloss
	leftBlock := lipgloss.NewStyle().Width(contentLeftWidth).Render(left)
	rightBlock := lipgloss.NewStyle().Width(contentRightWidth).Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Left, leftBlock, rightBlock)
}

func (m *model) IconOrSpinner(spinnerXpath config_attributes.Xpath, iconOnFinished string, tas *time_and_state.TimeAndState) string {
	var iconOrSpinner string

	if tas.HasStarted() && tas.IsFinished() {
		// Icon
		iconOrSpinner = iconOnFinished
		m.modelView.spinners.RemoveIfExists(spinnerXpath)
	} else if tas.HasStarted() && !tas.IsFinished() {
		// Spinner
		iconOrSpinner = m.modelView.spinners.GetOrCreateSpinner(spinnerXpath).View()
	}

	final := iconOrSpinner

	return final
}

func (m *model) DurationTas(logStyle config.ColorSchemeLogEntity, tas *time_and_state.TimeAndState) string {
	duration, err := tas.DurationOrElapsedTime()
	if err == nil {
		return m.DurationDas(logStyle, logs.DurationAndError{Duration: duration, Err: err})
	}

	return ""
}

func (m *model) DurationDas(logStyle config.ColorSchemeLogEntity, das logs.DurationAndError) string {
	return logStyle.Color.Render(fmt.Sprintf("(%.2fs)", das.Duration.Seconds()))
}

// Helpers

func nodeTitle(logStyle config.ColorSchemeLogEntity, attr *config_attributes.Attributes) string {
	return logStyle.Color.Render(fmt.Sprintf("%c %s %s", logStyle.Icon, attr.Name, attr.Message))
}
