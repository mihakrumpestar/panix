package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const (
	treeStep    = 3
	indentStep  = 2
	timerIndent = 4
)

var hideablePhases = []phases.Phase{phases.Inspect, phases.Secrets}

func (m *model) ViewBuildLogs() string {
	var builder strings.Builder

	builder.WriteString(m.conf.ColorScheme.Header.Title.Render("=== Build Logs ===\n"))

	selectedMachine := m.conf.Fleet.StatsTable.Selected
	selectedPhase := m.conf.Fleet.PhaseStatus.Selected
	colors := m.conf.ColorScheme

	for _, flakePair := range m.conf.Fleet.Flakes.Pairs() {
		flake := flakePair.Value
		if flake == nil {
			continue
		}
		flakeNode := m.createNode(0, colors.Flake, flake.Logs, true)

		for _, configurationPair := range flake.Configurations.Pairs() {
			configuration := configurationPair.Value
			if configuration == nil {
				continue
			}
			cfgNode := m.createNode(treeStep, colors.Configuration, configuration.Logs, false)

			switch {
			case selectedMachine.Index >= 0:
				for _, machinePair := range configuration.Machines.Pairs() {
					machine := machinePair.Value
					if machine == nil {
						continue
					}
					if machine.Xpath == selectedMachine.Xpath {
						m.addMachineTree(cfgNode, configuration, machine.Logs)

						break
					}
				}
			case selectedPhase.Index >= 0:
				m.addPhaseToTree(cfgNode, configuration.Logs, configuration.Machines, selectedPhase.Phase)
			default:
				m.addDefaultTree(cfgNode, configuration.Logs, configuration.Machines)
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

func (m *model) addPhaseToTree(treeNode *tree.Tree, cfgLog *logs.Logs, machines atomicorderedmap.AtomicOrderedMap[string, *machine.Machine], p phases.Phase) {
	indent := treeStep * indentStep

	if phases.GetPhaseScope(p) == phases.ScopeConfiguration {
		m.addPhases(treeNode, cfgLog, indent, false, p)
	} else {
		for _, machinePair := range machines.Pairs() {
			if machinePair.Value == nil || machinePair.Value.Logs == nil {
				continue
			}
			m.addMachinePhases(treeNode, machinePair.Value.Logs, indent, p)
		}
	}
}

func (m *model) addMachineTree(treeNode *tree.Tree, cfg *configuration.Configuration, machineLog *logs.Logs) {
	if machineLog == nil {
		return
	}
	indent := treeStep * indentStep
	node := m.createNode(indent, m.conf.ColorScheme.Machine, machineLog, false)

	errored := m.addPhases(node, machineLog, indent+treeStep, true, phases.Inspect)
	if !errored {
		for _, phaseMeta := range phases.PhaseRegistry[1:] {
			logNode := machineLog
			if phaseMeta.Scope == phases.ScopeConfiguration && cfg != nil {
				logNode = cfg.Logs
			}

			m.addPhases(node, logNode, indent+treeStep, true, phaseMeta.Phase)
		}
	}

	if node.Children().Length() > 0 {
		treeNode.Child(node)
	}
}

func (m *model) addDefaultTree(treeNode *tree.Tree, cfgLogs *logs.Logs, machines atomicorderedmap.AtomicOrderedMap[string, *machine.Machine]) {
	indent := treeStep * indentStep

	var pendingMachinePhases []phases.Phase

	flushMachinePhases := func() {
		if len(pendingMachinePhases) == 0 {
			return
		}

		for _, machinePair := range machines.Pairs() {
			if machinePair.Value == nil || machinePair.Value.Logs == nil {
				continue
			}
			m.addMachinePhases(treeNode, machinePair.Value.Logs, indent, pendingMachinePhases...)
		}

		pendingMachinePhases = nil
	}

	for _, phaseMeta := range phases.PhaseRegistry {
		if phaseMeta.Scope == phases.ScopeConfiguration {
			flushMachinePhases()
			m.addPhases(treeNode, cfgLogs, indent, false, phaseMeta.Phase)
		} else {
			pendingMachinePhases = append(pendingMachinePhases, phaseMeta.Phase)
		}
	}

	flushMachinePhases()
}

func (m *model) addMachinePhases(parent *tree.Tree, machineLogs *logs.Logs, indent int, allowed ...phases.Phase) {
	if machineLogs == nil {
		return
	}
	node := m.createNode(indent, m.conf.ColorScheme.Machine, machineLogs, false)
	m.addPhases(node, machineLogs, indent+treeStep, false, allowed...)

	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

func (m *model) createNode(indent int, style colorscheme.ColorSchemeLogEntity, logNode *logs.Logs, isRoot bool) *tree.Tree {
	var attr *attributes.Attributes
	durationSecs := 0.0

	if logNode != nil {
		attr = logNode.Attributes()
		durationSecs = logNode.DurationAndErrorCache.Duration.Seconds()
	}

	var name, message string
	if attr != nil {
		name = attr.Name
		message = attr.Message
	}

	leftRaw := fmt.Sprintf("%s %s %s", string(style.Icon), name, message)
	rightRaw := fmt.Sprintf(" (%.2fs)", durationSecs)

	left := style.Color.Render(leftRaw)
	right := style.Color.Render(rightRaw)

	line := m.layoutLine(indent, left, right, len(leftRaw), len(rightRaw))

	treeInst := tree.New().Root(line)
	if isRoot {
		treeInst = treeInst.Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(m.conf.ColorScheme.Tree.Enumerator).
			IndenterStyle(m.conf.ColorScheme.Tree.Enumerator)
	}

	return treeInst
}

func (m *model) addPhases(parent *tree.Tree, logNode *logs.Logs, indent int, stopAtError bool, allowed ...phases.Phase) bool {
	if logNode == nil || logNode.PhaseLogs == nil {
		return false
	}

	for _, pair := range logNode.PhaseLogs.Pairs() {
		if !slices.Contains(allowed, pair.Key) {
			continue
		}

		if m.addPhase(parent, logNode, pair.Key, pair.Value, indent) && stopAtError {
			return true
		}
	}

	return false
}

func (m *model) addPhase(parent *tree.Tree, logNode *logs.Logs, p phases.Phase, phaseLog *phase.PhaseLog, indent int) bool {
	if phaseLog == nil {
		return false
	}

	if m.shouldHidePhase(p, phaseLog) {
		return false
	}

	phaseNode := m.createPhaseNode(logNode, p, phaseLog, indent)
	hasError := m.addCommandsToPhase(phaseNode, phaseLog, p, logNode, indent)
	parent.Child(phaseNode)

	return hasError
}

func (m *model) shouldHidePhase(p phases.Phase, phaseLog *phase.PhaseLog) bool {
	hideable := slices.Contains(hideablePhases, p)
	shouldHide := (!m.conf.Flags.Tui.ShowAllBuildLogs && hideable) || m.conf.Flags.Tui.ShowActiveOnly

	tas := phaseLog.TimeAndState.Load()

	return shouldHide && tas.IsFinished() && tas.EndError == nil
}

func (m *model) createPhaseNode(logNode *logs.Logs, p phases.Phase, phaseLog *phase.PhaseLog, indent int) *tree.Tree {
	colors := m.conf.ColorScheme

	var phaseXpath xpath.Xpath
	if logNode != nil && logNode.Attributes() != nil {
		phaseXpath = logNode.Attributes().Xpath.NewXpathWithAppend(string(p))
	}

	tas := phaseLog.TimeAndState.Load()

	icon := m.spinnerOrIcon(phaseXpath, string(colors.Phase.Icon), tas)
	durationStyled, durationWidth := m.durationText(colors.Phase, tas)

	leftRaw := icon + strings.ToUpper(string(p))
	left := colors.Phase.Color.Render(leftRaw)

	line := m.layoutLine(indent, left, durationStyled, len(leftRaw), durationWidth)

	return tree.New().Root(colors.Phase.Color.Render(line))
}

func (m *model) addCommandsToPhase(phaseNode *tree.Tree, phaseLog *phase.PhaseLog, p phases.Phase, logNode *logs.Logs, indent int) bool {
	hideable := slices.Contains(hideablePhases, p)

	var phaseXpath xpath.Xpath
	if logNode != nil && logNode.Attributes() != nil {
		phaseXpath = logNode.Attributes().Xpath.NewXpathWithAppend(string(p))
	}

	cmds := phaseLog.CommandLogs
	if cmds == nil {
		return phaseLog.TimeAndState.Load().EndError != nil
	}

	hasError := phaseLog.TimeAndState.Load().EndError != nil

	for i, cmd := range cmds.Values() {
		if cmd == nil {
			continue
		}
		if m.conf.Flags.Tui.ShowAllBuildLogs || !hideable || i == cmds.Length()-1 {
			m.addCommand(phaseNode, cmd, i, phaseXpath, indent)

			if cmd.TimeAndState != nil && cmd.TimeAndState.Load() != nil && cmd.TimeAndState.Load().EndError != nil {
				hasError = true
			}
		}
	}

	return hasError
}

func (m *model) addCommand(parent *tree.Tree, cmd *command.CommandLog, idx int, phaseXpath xpath.Xpath, indent int) {
	if cmd == nil {
		return
	}
	colors := m.conf.ColorScheme
	cmdIndent := indent + treeStep
	resetable := m.resetable.Load()

	if resetable == nil {
		return
	}

	label := cmd.Description

	commandStr := cmd.Command
	if m.conf.Flags.Tui.ShowCommandsInLabels && len(commandStr) > 2 {
		label = commandStr
	}

	cmdXpath := phaseXpath.NewXpathWithAppend(label)
	tas := cmd.TimeAndState.Load()
	icon := m.spinnerOrIcon(cmdXpath, strconv.Itoa(idx+1), tas)
	durationStyled, durationWidth := m.durationText(colors.Command, tas)

	labelWidth := cmdIndent + lipgloss.Width(icon) + durationWidth
	labelViewport := resetable.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("label"), label, labelWidth)
	labelViewportHeight := lipgloss.Height(labelViewport)

	if labelViewportHeight > 1 {
		treeLine := "\n" + colors.Tree.Enumerator.Render("│")
		icon += strings.Repeat(treeLine, labelViewportHeight-1)
	}

	cmdNode := tree.New().Root(colors.Command.Color.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, icon, colors.Command.Color.Render(labelViewport), durationStyled),
	))

	output := cmd.StringForBuildLogs()
	if len(output) > 0 {
		cmdNode.Child(resetable.viewports.GetOrCreateViewport(cmdXpath.NewXpathWithAppend("output"), output, cmdIndent+treeStep*2-1))
	} else {
		resetable.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("output"))
	}

	err := tas.EndError
	if err != nil {
		errMsg := "✗ Command failed: " + err.Error()
		errViewport := resetable.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("error"), errMsg, cmdIndent+treeStep)
		cmdNode.Child(colors.Error.Color.Render(errViewport))
	} else {
		resetable.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("error"))
	}

	parent.Child(cmdNode)
}

func (m *model) layoutLine(indent int, left, right string, leftWidth, rightWidth int) string {
	leftWidth -= 2

	level := indent / treeStep
	timerIndentFromRight := timerIndent - level
	available := m.resetable.Load().viewports.ContentWidth() - indent - timerIndentFromRight
	centerSpace := strings.Repeat(" ", max(available-rightWidth-leftWidth, leftWidth))

	return left + centerSpace + right
}

func (m *model) spinnerOrIcon(xpath xpath.Xpath, icon string, tas *atomictimeandstate.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		return icon + " "
	}

	return m.resetable.Load().spinners.GetOrCreateSpinner(xpath).View()
}

func (m *model) durationText(style colorscheme.ColorSchemeLogEntity, tas *atomictimeandstate.TimeAndState) (string, int) {
	duration, err := tas.DurationOrElapsedTime()
	if err == nil {
		text := fmt.Sprintf(" (%.2fs)", duration.Seconds())
		styled := style.Color.Render(text)

		return styled, len(text)
	}

	return "", 0
}
