package buildlogs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

const (
	treeStep    = 3
	indentStep  = 2
	timerIndent = 4
)

var hideablePhases = []phases.Phase{phases.Inspect, phases.Secrets}

type BuildLogs struct {
	conf      *config.Config
	viewports *viewports.Viewports
	spinners  *spinners.Spinners
}

func New(conf *config.Config, viewports *viewports.Viewports, spinners *spinners.Spinners) *BuildLogs {
	return &BuildLogs{
		conf:      conf,
		viewports: viewports,
		spinners:  spinners,
	}
}

func (b *BuildLogs) View() string {
	var builder strings.Builder

	builder.WriteString(b.conf.ColorScheme.Header.Title.Render("=== Build Logs ===\n"))

	for _, flakePair := range b.conf.Fleet.Flakes.Pairs() {
		f := flakePair.Value
		if f == nil {
			continue
		}
		flakeNode := b.createNode(0, b.conf.ColorScheme.Flake, f.Logs, true)

		for _, cfgPair := range f.Configurations.Pairs() {
			cfg := cfgPair.Value
			if cfg == nil {
				continue
			}
			cfgNode := b.createNode(treeStep, b.conf.ColorScheme.Configuration, cfg.Logs, false)

			switch {
			case b.conf.Fleet.StatsTable.Selected.Index >= 0:
				for _, machinePair := range cfg.Machines.Pairs() {
					m := machinePair.Value
					if m == nil {
						continue
					}
					if m.Xpath == b.conf.Fleet.StatsTable.Selected.Xpath {
						b.addMachineTree(cfgNode, cfg, m.Logs)

						break
					}
				}
			case b.conf.Fleet.PhaseStatus.Selected.Index >= 0:
				b.addPhaseToTree(cfgNode, cfg.Logs, cfg.Machines, b.conf.Fleet.PhaseStatus.Selected.Phase)
			default:
				b.addDefaultTree(cfgNode, cfg.Logs, cfg.Machines)
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

func (b *BuildLogs) addPhaseToTree(
	treeNode *tree.Tree,
	cfgLog *logs.Logs,
	machines atomicorderedmap.AtomicOrderedMap[string, *machine.Machine],
	p phases.Phase,
) {
	indent := treeStep * indentStep

	if phases.GetPhaseScope(p) == phases.ScopeConfiguration {
		b.addPhases(treeNode, cfgLog, indent, false, p)
	} else {
		for _, machinePair := range machines.Pairs() {
			if machinePair.Value == nil || machinePair.Value.Logs == nil {
				continue
			}
			b.addMachinePhases(treeNode, machinePair.Value.Logs, indent, p)
		}
	}
}

func (b *BuildLogs) addMachineTree(
	treeNode *tree.Tree,
	cfg *configuration.Configuration,
	machineLog *logs.Logs,
) {
	if machineLog == nil {
		return
	}
	indent := treeStep * indentStep
	node := b.createNode(indent, b.conf.ColorScheme.Machine, machineLog, false)

	errored := b.addPhases(node, machineLog, indent+treeStep, true, phases.Inspect)
	if !errored {
		for _, phaseMeta := range phases.PhaseRegistry[1:] {
			logNode := machineLog
			if phaseMeta.Scope == phases.ScopeConfiguration && cfg != nil {
				logNode = cfg.Logs
			}

			b.addPhases(node, logNode, indent+treeStep, true, phaseMeta.Phase)
		}
	}

	if node.Children().Length() > 0 {
		treeNode.Child(node)
	}
}

func (b *BuildLogs) addDefaultTree(
	treeNode *tree.Tree,
	cfgLogs *logs.Logs,
	machines atomicorderedmap.AtomicOrderedMap[string, *machine.Machine],
) {
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
			b.addMachinePhases(treeNode, machinePair.Value.Logs, indent, pendingMachinePhases...)
		}

		pendingMachinePhases = nil
	}

	for _, phaseMeta := range phases.PhaseRegistry {
		if phaseMeta.Scope == phases.ScopeConfiguration {
			flushMachinePhases()
			b.addPhases(treeNode, cfgLogs, indent, false, phaseMeta.Phase)
		} else {
			pendingMachinePhases = append(pendingMachinePhases, phaseMeta.Phase)
		}
	}

	flushMachinePhases()
}

func (b *BuildLogs) addMachinePhases(
	parent *tree.Tree,
	machineLogs *logs.Logs,
	indent int,
	allowed ...phases.Phase,
) {
	if machineLogs == nil {
		return
	}
	node := b.createNode(indent, b.conf.ColorScheme.Machine, machineLogs, false)
	b.addPhases(node, machineLogs, indent+treeStep, false, allowed...)

	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

func (b *BuildLogs) createNode(indent int, style colorscheme.ColorSchemeLogEntity, logNode *logs.Logs, isRoot bool) *tree.Tree {
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

	line := b.layoutLine(indent, left, right, len(leftRaw), len(rightRaw))

	treeInst := tree.New().Root(line)
	if isRoot {
		treeInst = treeInst.Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(b.conf.ColorScheme.Tree.Enumerator).
			IndenterStyle(b.conf.ColorScheme.Tree.Enumerator)
	}

	return treeInst
}

func (b *BuildLogs) addPhases(
	parent *tree.Tree,
	logNode *logs.Logs,
	indent int,
	stopAtError bool,
	allowed ...phases.Phase,
) bool {
	if logNode == nil || logNode.PhaseLogs == nil {
		return false
	}

	for _, pair := range logNode.PhaseLogs.Pairs() {
		if !slices.Contains(allowed, pair.Key) {
			continue
		}

		if b.addPhase(parent, logNode, pair.Key, pair.Value, indent) && stopAtError {
			return true
		}
	}

	return false
}

func (b *BuildLogs) addPhase(
	parent *tree.Tree,
	logNode *logs.Logs,
	p phases.Phase,
	phaseLog *phase.PhaseLog,
	indent int,
) bool {
	if phaseLog == nil {
		return false
	}

	if b.shouldHidePhase(p, phaseLog) {
		return false
	}

	phaseNode := b.createPhaseNode(logNode, p, phaseLog, indent)
	hasError := b.addCommandsToPhase(phaseNode, phaseLog, p, logNode, indent)
	parent.Child(phaseNode)

	return hasError
}

func (b *BuildLogs) shouldHidePhase(p phases.Phase, phaseLog *phase.PhaseLog) bool {
	hideable := slices.Contains(hideablePhases, p)
	shouldHide := (!b.conf.Flags.Tui.ShowAllBuildLogs && hideable) || b.conf.Flags.Tui.ShowActiveOnly

	tas := phaseLog.TimeAndState.Load()

	return shouldHide && tas.IsFinished() && tas.EndError == nil
}

func (b *BuildLogs) createPhaseNode(logNode *logs.Logs, p phases.Phase, phaseLog *phase.PhaseLog, indent int) *tree.Tree {
	var phaseXpath xpath.Xpath
	if logNode != nil && logNode.Attributes() != nil {
		phaseXpath = logNode.Attributes().Xpath.NewXpathWithAppend(string(p))
	}

	tas := phaseLog.TimeAndState.Load()

	icon := b.spinnerOrIcon(phaseXpath, string(b.conf.ColorScheme.Phase.Icon), tas)
	durationStyled, durationWidth := b.durationText(b.conf.ColorScheme.Phase, tas)

	leftRaw := icon + strings.ToUpper(string(p))
	left := b.conf.ColorScheme.Phase.Color.Render(leftRaw)

	line := b.layoutLine(indent, left, durationStyled, len(leftRaw), durationWidth)

	return tree.New().Root(b.conf.ColorScheme.Phase.Color.Render(line))
}

func (b *BuildLogs) addCommandsToPhase(
	phaseNode *tree.Tree,
	phaseLog *phase.PhaseLog,
	p phases.Phase,
	logNode *logs.Logs,
	indent int,
) bool {
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
		if b.conf.Flags.Tui.ShowAllBuildLogs || !hideable || i == cmds.Length()-1 {
			b.addCommand(phaseNode, cmd, i, phaseXpath, indent)

			if cmd.TimeAndState != nil && cmd.TimeAndState.Load() != nil && cmd.TimeAndState.Load().EndError != nil {
				hasError = true
			}
		}
	}

	return hasError
}

func (b *BuildLogs) addCommand(parent *tree.Tree, cmd *command.CommandLog, idx int, phaseXpath xpath.Xpath, indent int) {
	if cmd == nil {
		return
	}
	cmdIndent := indent + treeStep

	if b.viewports == nil {
		return
	}

	label := cmd.Description

	commandStr := cmd.Command
	if b.conf.Flags.Tui.ShowCommandsInLabels && len(commandStr) > 2 {
		label = commandStr
	}

	cmdXpath := phaseXpath.NewXpathWithAppend(label)
	tas := cmd.TimeAndState.Load()
	icon := b.spinnerOrIcon(cmdXpath, strconv.Itoa(idx+1), tas)
	durationStyled, durationWidth := b.durationText(b.conf.ColorScheme.Command, tas)

	labelWidth := cmdIndent + lipgloss.Width(icon) + durationWidth
	labelViewport := b.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("label"), label, labelWidth)
	labelViewportHeight := lipgloss.Height(labelViewport)

	if labelViewportHeight > 1 {
		treeLine := "\n" + b.conf.ColorScheme.Tree.Enumerator.Render("│")
		icon += strings.Repeat(treeLine, labelViewportHeight-1)
	}

	cmdNode := tree.New().Root(b.conf.ColorScheme.Command.Color.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, icon, b.conf.ColorScheme.Command.Color.Render(labelViewport), durationStyled),
	))

	output := cmd.StringForBuildLogs()
	if len(output) > 0 {
		cmdNode.Child(b.viewports.GetOrCreateViewport(cmdXpath.NewXpathWithAppend("output"), output, cmdIndent+treeStep*2-1))
	} else {
		b.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("output"))
	}

	err := tas.EndError
	if err != nil {
		errMsg := "✗ Command failed: " + err.Error()
		errViewport := b.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("error"), errMsg, cmdIndent+treeStep)
		cmdNode.Child(b.conf.ColorScheme.Error.Color.Render(errViewport))
	} else {
		b.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("error"))
	}

	parent.Child(cmdNode)
}

func (b *BuildLogs) layoutLine(indent int, left, right string, leftWidth, rightWidth int) string {
	leftWidth -= 2

	level := indent / treeStep
	timerIndentFromRight := timerIndent - level
	available := b.viewports.ContentWidth() - indent - timerIndentFromRight
	centerSpace := strings.Repeat(" ", max(available-rightWidth-leftWidth, leftWidth))

	return left + centerSpace + right
}

func (b *BuildLogs) spinnerOrIcon(xp xpath.Xpath, icon string, tas *atomictimeandstate.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		return icon + " "
	}

	return b.spinners.GetOrCreateSpinner(xp).View()
}

func (b *BuildLogs) durationText(style colorscheme.ColorSchemeLogEntity, tas *atomictimeandstate.TimeAndState) (string, int) {
	duration, err := tas.DurationOrElapsedTime()
	if err == nil {
		text := fmt.Sprintf(" (%.2fs)", duration.Seconds())
		styled := style.Color.Render(text)

		return styled, len(text)
	}

	return "", 0
}
