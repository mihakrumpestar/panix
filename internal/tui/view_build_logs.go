package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_phase"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const treeStep = 3

var hideablePhases = []phases.Phase{phases.Inspect, phases.Secrets}

func (m *model) ViewBuildLogs() string {
	var builder strings.Builder
	state, colors := m.resetable.workflow.State(), m.conf.ColorScheme
	builder.WriteString(colors.HeaderTitle.Render("=== Build Logs ===\n"))

	selectedXpath := m.resetable.statsTable.GetSelectedXpath()
	stopAtError := selectedXpath.Depth() > 0

	for _, flakePair := range m.conf.Root.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		flakeNode := m.createNode(0, colors.Flake, &flake.Attributes, state, true)

		for _, cfgPair := range flake.Configurations.Omap.Pairs() {
			cfg := cfgPair.Value
			cfgNode := m.createNode(treeStep, colors.Configuration, &cfg.Attributes, state, false)
			machines := cfg.Machines.Omap.Pairs()

			if selectedXpath.Depth() > 0 {
				if m.renderSelectedMachine(cfgNode, cfg, state, colors, selectedXpath, stopAtError) {
					flakeNode.Child(cfgNode)
					builder.WriteString("\n" + flakeNode.String())
					return builder.String()
				}
				continue
			}

			for _, pair := range machines {
				m.addMachinePhases(cfgNode, pair.Value, colors, treeStep*2, phases.Inspect)
			}
			if len(machines) > 0 {
				m.addPhases(cfgNode, &machines[0].Value.Attributes, colors, treeStep*2, false, phases.Build)
			}
			for _, pair := range machines {
				m.addMachinePhases(cfgNode, pair.Value, colors, treeStep*2, phases.PhasesInOrder()[2:]...)
			}

			if cfgNode.Children().Length() > 0 {
				flakeNode.Child(cfgNode)
			}
		}

		if flakeNode.Children().Length() > 0 {
			builder.WriteString("\n" + flakeNode.String())
		}
	}

	return builder.String()
}

func (m *model) renderSelectedMachine(cfgNode *tree.Tree, cfg *config.Configuration, state *workflow.WorkflowState, colors *config.ColorScheme, selectedXpath config_attributes.Xpath, stopAtError bool) bool {
	for _, pair := range cfg.Machines.Omap.Pairs() {
		machine := pair.Value
		if machine.Xpath != selectedXpath {
			continue
		}

		indent := treeStep * 2
		machineNode := m.createNode(indent, colors.Machine, &machine.Attributes, state, false)
		errored := m.addPhases(machineNode, &machine.Attributes, colors, indent+treeStep, stopAtError, phases.Inspect)

		if !errored {
			m.addPhases(machineNode, &cfg.Attributes, colors, indent+treeStep, stopAtError, phases.Build)
			m.addPhases(machineNode, &machine.Attributes, colors, indent+treeStep, stopAtError, phases.PhasesInOrder()[2:]...)
		}

		if machineNode.Children().Length() > 0 {
			cfgNode.Child(machineNode)
		}
		return true
	}
	return false
}

func (m *model) addMachinePhases(parent *tree.Tree, machine *config.Machine, colors *config.ColorScheme, indent int, allowed ...phases.Phase) {
	node := m.createNode(indent, colors.Machine, &machine.Attributes, m.resetable.workflow.State(), false)
	m.addPhases(node, &machine.Attributes, colors, indent+treeStep, false, allowed...)
	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

func (m *model) createNode(indent int, style config.ColorSchemeLogEntity, attr *config_attributes.Attributes, state *workflow.WorkflowState, isRoot bool) *tree.Tree {
	duration := state.TargetsLogs.Get(attr.Xpath).CalculateDurationAndError().Duration
	line := m.layoutLine(indent, style.Color.Render(fmt.Sprintf("%c %s %s", style.Icon, attr.Name, attr.Message)),
		style.Color.Render(fmt.Sprintf("(%.2fs)", duration.Seconds())))

	t := tree.New().Root(line)
	if isRoot {
		t = t.Enumerator(tree.RoundedEnumerator).EnumeratorStyle(m.conf.ColorScheme.TreeEnumerator)
	}
	return t
}

func (m *model) addPhases(parent *tree.Tree, attr *config_attributes.Attributes, colors *config.ColorScheme, indent int, stopAtError bool, allowed ...phases.Phase) bool {
	logs := m.resetable.workflow.State().TargetsLogs.GetLogs(attr.Xpath)

	for _, entry := range logs.All() {
		phase := entry.Key
		if !slices.Contains(allowed, phase) {
			continue
		}

		if hasError := m.addPhase(parent, attr, phase, entry.Value, colors, indent); hasError && stopAtError {
			return true
		}
	}
	return false
}

func (m *model) addPhase(parent *tree.Tree, attr *config_attributes.Attributes, phase phases.Phase, phaseLog *logs_phase.PhaseLog, colors *config.ColorScheme, indent int) bool {
	if phaseLog == nil {
		return false
	}

	tas := phaseLog.TimeAndState()
	if !m.conf.Flags.Tui.ShowAllBuildLogs && slices.Contains(hideablePhases, phase) && tas.IsFinished() && tas.GetEndError() == nil {
		return false
	}
	if m.conf.Flags.Tui.ShowActiveOnly && tas.IsFinished() && tas.GetEndError() == nil {
		return false
	}

	phaseXpath := attr.Xpath.NewXpathWithAppend(string(phase))
	icon := m.spinnerOrIcon(phaseXpath, string(colors.Phase.Icon), tas)
	line := m.layoutLine(indent, colors.Phase.Color.Render(icon+" "+strings.ToUpper(string(phase))),
		m.durationText(colors.Phase, tas))
	phaseNode := tree.New().Root(colors.Phase.Color.Render(line))

	cmds := phaseLog.CommandLogs()
	hasError := false
	for i, cmd := range cmds {
		if m.conf.Flags.Tui.ShowAllBuildLogs || !slices.Contains(hideablePhases, phase) || i == len(cmds)-1 {
			m.addCommand(phaseNode, cmd, i, phaseXpath, colors, indent)
			if cmd.TimeAndState.GetEndError() != nil {
				hasError = true
			}
		}
	}
	parent.Child(phaseNode)

	return hasError || tas.GetEndError() != nil
}

func (m *model) addCommand(parent *tree.Tree, cmd *logs_command.CommandLog, idx int, phaseXpath config_attributes.Xpath, colors *config.ColorScheme, indent int) {
	cmdIndent := indent + treeStep
	label := strings.TrimSpace(cmd.Description)
	if m.conf.Flags.Tui.ShowCommandsInLabels {
		label = cmd.Command
	}

	cmdXpath := phaseXpath.NewXpathWithAppend(label)
	icon := m.spinnerOrIcon(cmdXpath, fmt.Sprintf("%d ", idx+1), cmd.TimeAndState)
	duration := m.durationText(colors.Command, cmd.TimeAndState)
	labelViewport := m.resetable.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("label"), label, cmdIndent+lipgloss.Width(icon)+lipgloss.Width(duration))

	output := strings.TrimSpace(cmd.String())
	if hasOutput := len(output) > 0; hasOutput && lipgloss.Height(labelViewport) > 1 {
		icon = lipgloss.JoinVertical(lipgloss.Left, append([]string{icon},
			slices.Repeat([]string{colors.TreeEnumerator.Render("│")}, lipgloss.Height(labelViewport)-1)...)...)
	}

	cmdNode := tree.New().Root(colors.Command.Color.Render(lipgloss.JoinHorizontal(lipgloss.Top, icon, colors.Command.Color.Render(labelViewport), duration)))
	if output := strings.TrimSpace(cmd.String()); len(output) > 0 {
		cmdNode.Child(m.resetable.viewports.GetOrCreateViewport(cmdXpath.NewXpathWithAppend("output"), output, cmdIndent+treeStep*2-1))
	} else {
		m.resetable.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("output"))
	}

	if err := cmd.TimeAndState.GetEndError(); err != nil {
		cmdNode.Child(colors.Error.Color.Render(fmt.Sprintf("✗ Command failed: %s", err.Error())))
	}

	parent.Child(cmdNode)
}

func (m *model) layoutLine(indent int, left, right string) string {
	timerIndent := max(4-indent/treeStep, 0)
	available := m.resetable.viewports.ContentWidth() - indent - timerIndent
	rightWidth := lipgloss.Width(right)
	return lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(max(available-rightWidth, lipgloss.Width(left))).Render(left),
		right)
}

func (m *model) spinnerOrIcon(xpath config_attributes.Xpath, icon string, tas *time_and_state.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}
	if tas.IsFinished() {
		return icon
	}
	return m.resetable.spinners.GetOrCreateSpinner(xpath).View()
}

func (m *model) durationText(style config.ColorSchemeLogEntity, tas *time_and_state.TimeAndState) string {
	if d, err := tas.DurationOrElapsedTime(); err == nil {
		return style.Color.PaddingLeft(1).Render(fmt.Sprintf("(%.2fs)", d.Seconds()))
	}
	return ""
}
