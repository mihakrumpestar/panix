package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/timeandstate"
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

	m.conf.Fleet.CalculateDurationAndError(m.conf.Phases)

	resetable := m.resetable.Load()
	selectedXpath := resetable.statsTable.GetSelectedXpath()
	selectedPhase := resetable.phaseStatus.GetSelectedPhase()
	colors := m.conf.ColorScheme

	for _, flake := range m.conf.Fleet.Flakes {
		flakeNode := m.createNode(0, colors.Flake, flake, true)

		for _, cfg := range flake.Configurations {
			cfgNode := m.createNode(treeStep, colors.Configuration, cfg, false)

			switch {
			case selectedXpath.Depth() > 0:
				for _, machine := range cfg.Machines {
					if machine.Xpath == selectedXpath {
						m.addMachineTree(cfgNode, cfg, machine)

						break
					}
				}
			case selectedPhase != "":
				m.addPhaseToTree(cfgNode, cfg, cfg.Machines, selectedPhase)
			default:
				m.addDefaultTree(cfgNode, cfg, cfg.Machines)
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

func (m *model) addPhaseToTree(cfgNode *tree.Tree, cfg *config.Configuration, machines []*config.Machine, p phases.Phase) {
	indent := treeStep * indentStep

	if phases.GetPhaseScope(p) == phases.ScopeConfig {
		m.addPhases(cfgNode, cfg, indent, false, p)
	} else {
		for _, machine := range machines {
			m.addMachinePhases(cfgNode, machine, indent, p)
		}
	}
}

func (m *model) addMachineTree(cfgNode *tree.Tree, cfg *config.Configuration, machine *config.Machine) {
	indent := treeStep * indentStep
	node := m.createNode(indent, m.conf.ColorScheme.Machine, machine, false)

	errored := m.addPhases(node, machine, indent+treeStep, true, phases.Inspect)
	if !errored {
		for _, phaseMeta := range phases.PhaseRegistry[1:] {
			logNode := config.LogNode(machine)
			if phaseMeta.Scope == phases.ScopeConfig {
				logNode = cfg
			}

			m.addPhases(node, logNode, indent+treeStep, true, phaseMeta.Phase)
		}
	}

	if node.Children().Length() > 0 {
		cfgNode.Child(node)
	}
}

func (m *model) addDefaultTree(cfgNode *tree.Tree, cfg *config.Configuration, machines []*config.Machine) {
	indent := treeStep * indentStep

	var pendingMachinePhases []phases.Phase

	flushMachinePhases := func() {
		if len(pendingMachinePhases) == 0 {
			return
		}

		for _, machine := range machines {
			m.addMachinePhases(cfgNode, machine, indent, pendingMachinePhases...)
		}

		pendingMachinePhases = nil
	}

	for _, phaseMeta := range phases.PhaseRegistry {
		if phaseMeta.Scope == phases.ScopeConfig {
			flushMachinePhases()
			m.addPhases(cfgNode, cfg, indent, false, phaseMeta.Phase)
		} else {
			pendingMachinePhases = append(pendingMachinePhases, phaseMeta.Phase)
		}
	}

	flushMachinePhases()
}

func (m *model) addMachinePhases(parent *tree.Tree, machine *config.Machine, indent int, allowed ...phases.Phase) {
	node := m.createNode(indent, m.conf.ColorScheme.Machine, machine, false)
	m.addPhases(node, machine, indent+treeStep, false, allowed...)

	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

func (m *model) createNode(indent int, style config.ColorSchemeLogEntity, logNode config.LogNode, isRoot bool) *tree.Tree {
	duration := logNode.GetCachedDurationAndError().Duration

	attr := m.logNodeAttrs(logNode)

	leftRaw := fmt.Sprintf("%s %s %s", string(style.Icon), attr.Name, attr.Message)
	rightRaw := fmt.Sprintf(" (%.2fs)", duration.Seconds())

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

func (m *model) logNodeAttrs(logNode config.LogNode) *attributes.Attributes {
	switch n := logNode.(type) {
	case *config.Fleet:
		return &n.Attributes
	case *config.Flake:
		return &n.Attributes
	case *config.Configuration:
		return &n.Attributes
	case *config.Machine:
		return &n.Attributes
	default:
		return nil
	}
}

func (m *model) addPhases(parent *tree.Tree, logNode config.LogNode, indent int, stopAtError bool, allowed ...phases.Phase) bool {
	logs := logNode.GetLogs()

	for _, entry := range logs.All() {
		if !slices.Contains(allowed, entry.Key) {
			continue
		}

		if m.addPhase(parent, logNode, entry.Key, entry.Value, indent) && stopAtError {
			return true
		}
	}

	return false
}

func (m *model) addPhase(parent *tree.Tree, logNode config.LogNode, p phases.Phase, phaseLog *phase.PhaseLog, indent int) bool {
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

	return shouldHide && phaseLog.TimeAndState().IsFinished() && phaseLog.TimeAndState().GetEndError() == nil
}

func (m *model) createPhaseNode(logNode config.LogNode, p phases.Phase, phaseLog *phase.PhaseLog, indent int) *tree.Tree {
	colors := m.conf.ColorScheme
	attr := m.logNodeAttrs(logNode)
	phaseXpath := attr.Xpath.NewXpathWithAppend(string(p))
	tas := phaseLog.TimeAndState()

	icon := m.spinnerOrIcon(phaseXpath, string(colors.Phase.Icon), tas)
	durationStyled, durationWidth := m.durationText(colors.Phase, tas)

	leftRaw := icon + strings.ToUpper(string(p))
	left := colors.Phase.Color.Render(leftRaw)

	line := m.layoutLine(indent, left, durationStyled, len(leftRaw), durationWidth)

	return tree.New().Root(colors.Phase.Color.Render(line))
}

func (m *model) addCommandsToPhase(phaseNode *tree.Tree, phaseLog *phase.PhaseLog, p phases.Phase, logNode config.LogNode, indent int) bool {
	hideable := slices.Contains(hideablePhases, p)
	attr := m.logNodeAttrs(logNode)
	phaseXpath := attr.Xpath.NewXpathWithAppend(string(p))
	cmds := phaseLog.CommandLogs()
	hasError := phaseLog.TimeAndState().GetEndError() != nil

	for i, cmd := range cmds {
		if m.conf.Flags.Tui.ShowAllBuildLogs || !hideable || i == len(cmds)-1 {
			m.addCommand(phaseNode, cmd, i, phaseXpath, indent)

			if cmd.TimeAndState.GetEndError() != nil {
				hasError = true
			}
		}
	}

	return hasError
}

func (m *model) addCommand(parent *tree.Tree, cmd *command.CommandLog, idx int, phaseXpath attributes.Xpath, indent int) {
	colors := m.conf.ColorScheme
	cmdIndent := indent + treeStep
	resetable := m.resetable.Load()

	label := cmd.Description

	command := cmd.Command
	if m.conf.Flags.Tui.ShowCommandsInLabels && len(command) > 2 {
		label = command
	}

	cmdXpath := phaseXpath.NewXpathWithAppend(label)
	icon := m.spinnerOrIcon(cmdXpath, strconv.Itoa(idx+1), cmd.TimeAndState)
	durationStyled, durationWidth := m.durationText(colors.Command, cmd.TimeAndState)

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

	err := cmd.TimeAndState.GetEndError()
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

func (m *model) spinnerOrIcon(xpath attributes.Xpath, icon string, tas *timeandstate.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		return icon + " "
	}

	return m.resetable.Load().spinners.GetOrCreateSpinner(xpath).View()
}

func (m *model) durationText(style config.ColorSchemeLogEntity, tas *timeandstate.TimeAndState) (string, int) {
	duration, err := tas.DurationOrElapsedTime()
	if err == nil {
		text := fmt.Sprintf(" (%.2fs)", duration.Seconds())
		styled := style.Color.Render(text)

		return styled, len(text)
	}

	return "", 0
}
