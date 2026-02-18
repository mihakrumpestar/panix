package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/config_attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/logs_command"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const (
	treeStep  = 3
	timerStep = 1
)

var hideablePhases = []phases.Phase{phases.Inspect, phases.Secrets}

func (m *model) ViewBuildLogs() string {
	var builder strings.Builder
	state, colors := m.resetable.workflow.State(), m.conf.ColorScheme
	builder.WriteString(colors.HeaderTitle.Render("=== Build Logs ===\n"))
	for _, pair := range m.conf.Root.Flakes.Omap.Pairs() {
		m.renderFlake(&builder, pair.Value, state, colors)
	}

	return builder.String()
}

func (m *model) renderFlake(builder *strings.Builder, flake *config.Flake, state *workflow.WorkflowState, colors *config.ColorScheme) {
	node := tree.New().Root(m.entityLine(0, colors.Flake, &flake.Attributes, state.TargetsLogs.Get(flake.Xpath).CalculateDurationAndError().Duration)).
		Enumerator(tree.RoundedEnumerator).EnumeratorStyle(colors.TreeEnumerator)
	for _, pair := range flake.Configurations.Omap.Pairs() {
		m.renderConfig(node, pair.Value, state, colors, treeStep)
	}
	builder.WriteString("\n" + node.String())
}

func (m *model) renderConfig(parent *tree.Tree, cfg *config.Configuration, state *workflow.WorkflowState, colors *config.ColorScheme, indent int) {
	node := tree.New().Root(m.entityLine(indent, colors.Configuration, &cfg.Attributes, state.TargetsLogs.Get(cfg.Xpath).CalculateDurationAndError().Duration))
	parent.Child(node)
	indent += treeStep
	machines := cfg.Machines.Omap.Pairs()
	for _, pair := range machines {
		m.renderMachine(node, pair.Value, colors, indent, phases.Inspect)
	}
	if len(machines) > 0 {
		m.addPhases(node, &machines[0].Value.Attributes, colors, indent, phases.Build)
	}
	for _, pair := range machines {
		m.renderMachine(node, pair.Value, colors, indent, phases.PhasesInOrder()[2:]...)
	}
}

func (m *model) renderMachine(parent *tree.Tree, machine *config.Machine, colors *config.ColorScheme, indent int, allowedPhases ...phases.Phase) {
	state := m.resetable.workflow.State()
	node := tree.New().Root(m.entityLine(indent, colors.Machine, &machine.Attributes, state.TargetsLogs.Get(machine.Xpath).CalculateDurationAndError().Duration))
	m.addPhases(node, &machine.Attributes, colors, indent+treeStep, allowedPhases...)
	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

func (m *model) addPhases(parent *tree.Tree, attr *config_attributes.Attributes, colors *config.ColorScheme, indent int, allowed ...phases.Phase) {
	logs := m.resetable.workflow.State().TargetsLogs.GetLogs(attr.Xpath)
	for _, entry := range logs.All() {
		phase, phaseLog := entry.Key, entry.Value
		if !slices.Contains(allowed, phase) {
			continue
		}
		timeAndState := phaseLog.TimeAndState()
		if !m.conf.Flags.Tui.ShowAllBuildLogs && slices.Contains(hideablePhases, phase) && timeAndState.IsFinished() && timeAndState.GetEndError() == nil {
			continue
		}
		if m.conf.Flags.Tui.ShowActiveOnly && timeAndState.IsFinished() && timeAndState.GetEndError() == nil {
			continue
		}
		phaseXpath := attr.Xpath.NewXpathWithAppend(string(phase))
		phaseNode := tree.New().Root(colors.Phase.Color.Render(m.layoutLine(indent, calcTimerIndent(indent),
			m.spinnerOrIcon(phaseXpath, string(colors.Phase.Icon), timeAndState)+" "+strings.ToUpper(string(phase)),
			m.durationText(colors.Phase, timeAndState))))
		cmds := phaseLog.CommandLogs()
		for i, cmd := range cmds {
			if m.conf.Flags.Tui.ShowAllBuildLogs || !slices.Contains(hideablePhases, phase) || i == len(cmds)-1 {
				m.addCommand(phaseNode, cmd, i, phaseXpath, colors, indent)
			}
		}
		parent.Child(phaseNode)
	}
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
	hasOutput := len(output) > 0
	if hasOutput && lipgloss.Height(labelViewport) > 1 {
		icon = lipgloss.JoinVertical(lipgloss.Left, append([]string{icon}, slices.Repeat([]string{colors.TreeEnumerator.Render("│")}, lipgloss.Height(labelViewport)-1)...)...)
	}
	cmdNode := tree.New().Root(colors.Command.Color.Render(lipgloss.JoinHorizontal(lipgloss.Top, icon, colors.Command.Color.Render(labelViewport), duration)))
	if hasOutput {
		cmdNode.Child(m.resetable.viewports.GetOrCreateViewport(cmdXpath.NewXpathWithAppend("output"), output, cmdIndent+treeStep*2-1))
	} else {
		m.resetable.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("output"))
	}
	if err := cmd.TimeAndState.GetEndError(); err != nil {
		cmdNode.Child(colors.Error.Color.Render(fmt.Sprintf("✗ Command failed: %s", err.Error())))
	}
	parent.Child(cmdNode)
}

func calcTimerIndent(indent int) int {
	return max(timerStep*4-indent/treeStep*timerStep, 0)
}

func (m *model) entityLine(indent int, style config.ColorSchemeLogEntity, attr *config_attributes.Attributes, duration time.Duration) string {
	return m.layoutLine(indent, calcTimerIndent(indent),
		style.Color.Render(fmt.Sprintf("%c %s %s", style.Icon, attr.Name, attr.Message)),
		style.Color.Render(fmt.Sprintf("(%.2fs)", duration.Seconds())))
}

func (m *model) layoutLine(indent, timerIndent int, left, right string) string {
	available := m.resetable.viewports.ContentWidth() - indent - timerIndent
	rightWidth := lipgloss.Width(right)
	return lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(max(available-rightWidth, lipgloss.Width(left))).Render(left),
		right)
}

func (m *model) spinnerOrIcon(xpath config_attributes.Xpath, icon string, timeAndState *time_and_state.TimeAndState) string {
	if !timeAndState.HasStarted() {
		return ""
	}
	if timeAndState.IsFinished() {
		return icon
	}
	return m.resetable.spinners.GetOrCreateSpinner(xpath).View()
}

func (m *model) durationText(style config.ColorSchemeLogEntity, timeAndState *time_and_state.TimeAndState) string {
	if d, err := timeAndState.DurationOrElapsedTime(); err == nil {
		return style.Color.PaddingLeft(1).Render(fmt.Sprintf("(%.2fs)", d.Seconds()))
	}
	return ""
}
