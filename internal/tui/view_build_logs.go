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
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// BuildLogHierarchy determines how logs are organized in the tree view
type BuildLogHierarchy int

const (
	PhaseFirstMachineSecond BuildLogHierarchy = iota
	MachineFirstPhaseSecond
	PhaseOnly

	treeInitIndent int = 0
	treeStep       int = 3
)

// ViewBuildLogs generates the Docker-style build log view with tree structure
func (m *model) ViewBuildLogs() string {
	var builder strings.Builder
	state := m.workflow.State()
	colors := state.Conf.ColorScheme

	builder.WriteString(colors.HeaderTitle.Render("=== Build Logs ===\n"))

	for _, flakePair := range state.Conf.Root.Flakes.Omap.Pairs() {
		m.renderFlakeNode(&builder, flakePair.Value, state, colors, treeInitIndent)
	}

	builder.WriteString("\n\n")
	return builder.String()
}

func (m *model) renderFlakeNode(builder *strings.Builder, flake *config.Flake, state *workflow.WorkflowState, colors *config.ColorScheme, prefixLen int) {
	title := m.MostLeftAndMostRight(prefixLen,
		nodeTitle(colors.Flake, &flake.Attributes),
		m.DurationDas(colors.Flake, state.Conf.TargetsLogs.Get(flake.Xpath).CalculateDurationAndError()),
	)

	flakeNode := tree.New().
		Root(title).
		Enumerator(tree.RoundedEnumerator).
		EnumeratorStyle(colors.TreeEnumerator)

	childPrefixLen := prefixLen + treeStep
	for _, configPair := range flake.Configurations.Omap.Pairs() {
		m.renderConfigurationNode(flakeNode, configPair.Value, state, colors, childPrefixLen)
	}

	builder.WriteString("\n" + flakeNode.String())
}

func (m *model) renderConfigurationNode(flakeNode *tree.Tree, configuration *config.Configuration, state *workflow.WorkflowState, colors *config.ColorScheme, prefixLen int) {
	title := m.MostLeftAndMostRight(prefixLen,
		nodeTitle(colors.Configuration, &configuration.Attributes),
		m.DurationDas(colors.Configuration, state.Conf.TargetsLogs.Get(configuration.Xpath).CalculateDurationAndError()),
	)

	configurationNode := tree.New().Root(title)
	flakeNode.Child(configurationNode)

	m.forMachines(prefixLen, configurationNode, configuration, func(prefixLenScoped int, machine *config.Machine, machineNode *tree.Tree) {
		m.addChildLogs(prefixLenScoped, &machine.Attributes, machineNode, colors, PhaseFirstMachineSecond, phases.Inspect)
	})

	if firstMachine := configuration.Machines.Omap.Pairs(); len(firstMachine) > 0 {
		m.addChildLogs(prefixLen, &firstMachine[0].Value.Attributes, configurationNode, colors, PhaseOnly, phases.Build)
	}

	m.forMachines(prefixLen, configurationNode, configuration, func(prefixLenScoped int, machine *config.Machine, machineNode *tree.Tree) {
		m.addChildLogs(prefixLenScoped, &machine.Attributes, machineNode, colors, MachineFirstPhaseSecond, phases.PhasesInOrder()[2:]...)
	})
}

func (m *model) forMachines(prefixLen int, configurationNode *tree.Tree, configuration *config.Configuration, f func(prefixLenScoped int, machine *config.Machine, machineNode *tree.Tree)) {
	colors := m.workflow.State().Conf.ColorScheme

	prefixLen += treeStep
	for _, machinePair := range configuration.Machines.Omap.Pairs() {
		machine := machinePair.Value
		title := m.MostLeftAndMostRight(prefixLen,
			nodeTitle(colors.Machine, &machine.Attributes),
			m.DurationDas(colors.Machine, m.workflow.State().Conf.TargetsLogs.Get(machine.Xpath).CalculateDurationAndError()),
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

	if logs.Len() == 0 {
		return
	}

	prefixLen += treeStep

	phaseNodes := m.phaseNodes(prefixLen, attr.Xpath, logs, colors, hierarchy, limitToPhases...)
	for _, phaseNode := range phaseNodes {
		treeRoot.Child(phaseNode)
	}
}

func (m *model) phaseNodes(prefixLen int, xpath config_attributes.Xpath, phaseLogs *logs_phase.PhaseLogs, colors *config.ColorScheme, hierarchy BuildLogHierarchy, limitToPhases ...phases.Phase) []*tree.Tree {
	phaseNodes := make([]*tree.Tree, 0, phaseLogs.Len())

	for _, entry := range phaseLogs.All() {
		phase := entry.Key
		phaseLog := entry.Value

		if !slices.Contains(limitToPhases, phase) {
			continue
		}

		phaseXpath := xpath.NewXpathWithAppend(string(phase))
		phaseTree, _ := m.phaseLogs(prefixLen, phaseLog, colors, phaseXpath)

		cmds := phaseLog.CommandLogs()
		for cmdIdx, cmd := range cmds {
			m.renderCommandNode(phaseTree, prefixLen, phase, cmdIdx, len(cmds), cmd, phaseXpath, colors)
		}

		phaseNodes = append(phaseNodes, phaseTree)
	}

	return phaseNodes
}

func (m *model) renderCommandNode(phaseTree *tree.Tree, prefixLen int, phase phases.Phase, cmdIdx int, totalCmds int, cmd *logs_command.CommandLog, phaseXpath config_attributes.Xpath, colors *config.ColorScheme) {
	prefixLenCmd := prefixLen + treeStep

	cmdLabel := cmd.Description
	cmdOutput := cmd.String()
	cmdTas := cmd.TimeAndState

	if !m.workflow.State().Conf.Flags.Tui.ShowAllBuildLogs && slices.Contains([]phases.Phase{phases.Inspect, phases.Secrets}, phase) && cmdTas.GetEndError() == nil {
		if cmdIdx == totalCmds-1 {
			cmdLabel = "(hidden)"
			cmdOutput = ""
		} else {
			return
		}
	}

	iconOnFinished := fmt.Sprintf("%d ", cmdIdx+1)
	commandXpath := phaseXpath.NewXpathWithAppend(cmdLabel)
	labelXpath := commandXpath.NewXpathWithAppend("label")

	leftIcon := m.IconOrSpinner(commandXpath, iconOnFinished, cmdTas)
	duration := m.DurationTas(colors.Command, cmdTas)

	iconWidth := lipgloss.Width(leftIcon)
	durationWidth := lipgloss.Width(duration)
	reservedWidth := iconWidth + durationWidth
	adjustedPrefixLenCmd := prefixLenCmd + reservedWidth

	cmdLabelViewport := m.modelView.viewports.GetOrCreateLabelViewport(labelXpath, cmdLabel, adjustedPrefixLenCmd)

	entry := leftIcon + cmdLabelViewport + duration
	cmdHeader := colors.Command.Color.Render(entry)
	cmdTree := tree.New().Root(cmdHeader)

	viewportXpath := commandXpath.NewXpathWithAppend("output")
	output := strings.TrimSpace(cmdOutput)
	if len(output) != 0 {
		outputViewport := m.modelView.viewports.GetOrCreateViewport(viewportXpath, output, prefixLenCmd+treeStep*2-1)
		cmdTree.Child(outputViewport)
	} else {
		m.modelView.viewports.RemoveIfExistsViewport(viewportXpath)
	}

	if err := cmdTas.GetEndError(); err != nil {
		cmdTree.Child(colors.Error.Color.Render(fmt.Sprintf("✗ Command failed: %s", err.Error())))
	}

	phaseTree.Child(cmdTree)
}

func (m *model) phaseLogs(prefixLen int, phaseLog *logs_phase.PhaseLog, colors *config.ColorScheme, xpath config_attributes.Xpath) (*tree.Tree, error) {
	tas := phaseLog.TimeAndState()

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
	availableWidth := m.modelView.viewports.ContentWidth() - prefixLen

	contentRightWidth := lipgloss.Width(right)
	contentLeftWidth := max(availableWidth-contentRightWidth, lipgloss.Width(left))

	leftBlock := lipgloss.NewStyle().Width(contentLeftWidth).Render(left)
	rightBlock := lipgloss.NewStyle().Width(contentRightWidth).Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Left, leftBlock, rightBlock)
}

func (m *model) IconOrSpinner(spinnerXpath config_attributes.Xpath, iconOnFinished string, tas *time_and_state.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		m.modelView.spinners.RemoveIfExists(spinnerXpath)
		return iconOnFinished
	}

	return m.modelView.spinners.GetOrCreateSpinner(spinnerXpath).View()
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

func nodeTitle(logStyle config.ColorSchemeLogEntity, attr *config_attributes.Attributes) string {
	return logStyle.Color.Render(fmt.Sprintf("%c %s %s", logStyle.Icon, attr.Name, attr.Message))
}
