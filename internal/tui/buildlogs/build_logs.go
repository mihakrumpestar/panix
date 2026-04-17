package buildlogs

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/command"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

const (
	treeStep    = 3
	indentStep  = 2
	timerIndent = 4
	machineInd  = treeStep * indentStep
	phaseInd    = machineInd + treeStep
)

var hideablePhases = []phase.Phase{phase.Inspect, phase.Secrets}

type BuildLogs struct {
	conf *config.Config

	viewports *viewports.Viewports
	spinners  *spinners.Spinners
}

func New(conf *config.Config) *BuildLogs {
	return &BuildLogs{conf: conf}
}

func (b *BuildLogs) View(vp *viewports.Viewports, sp *spinners.Spinners) string {
	b.viewports = vp
	b.spinners = sp

	var sb strings.Builder
	sb.WriteString(b.conf.ColorScheme.Header.Title.Render("=== Build Logs ===\n"))

	for _, fp := range b.conf.Fleet.Flakes.Pairs() {
		f := fp.Value
		if f == nil {
			continue
		}

		flakeNode := b.entityNode(0, b.conf.ColorScheme.Flake, f.Name, f.Logs, true)

		for _, cp := range f.Configurations.Pairs() {
			cfg := cp.Value
			if cfg == nil {
				continue
			}

			cfgNode := b.entityNode(treeStep, b.conf.ColorScheme.Configuration, cfg.Name, cfg.Logs, false)
			b.buildConfigTree(cfgNode, cfg)

			if cfgNode.Children().Length() > 0 {
				flakeNode.Child(cfgNode)
			}
		}

		if flakeNode.Children().Length() > 0 {
			sb.WriteString("\n" + flakeNode.String())
		}
	}

	return sb.String()
}

func (b *BuildLogs) buildConfigTree(cfgNode *tree.Tree, cfg *configuration.Configuration) {
	switch {
	case b.conf.Fleet.StatsTable.Selected.Index >= 0:
		b.buildMachineSelectedTree(cfgNode, cfg)
	case b.conf.Fleet.PhaseStatus.Selected.Index >= 0:
		b.buildPhaseSelectedTree(cfgNode, cfg)
	default:
		b.buildDefaultTree(cfgNode, cfg)
	}
}

func (b *BuildLogs) buildMachineSelectedTree(cfgNode *tree.Tree, cfg *configuration.Configuration) {
	for _, mp := range cfg.Machines.Pairs() {
		m := mp.Value
		if m == nil || m.Xpath != b.conf.Fleet.StatsTable.Selected.Xpath {
			continue
		}

		if m.Logs == nil {
			return
		}

		node := b.entityNode(machineInd, b.conf.ColorScheme.Machine, m.Name, m.Logs, false)

		errored := b.addPhases(node, m.Logs, m.Xpath, phaseInd, true, phase.Inspect)
		if !errored {
			for _, pm := range phase.PhaseRegistry[1:] {
				logs_, xp := b.phaseLogsAndXpath(pm, cfg, m)
				if b.addPhases(node, logs_, xp, phaseInd, true, pm.Phase) {
					break
				}
			}
		}

		if node.Children().Length() > 0 {
			cfgNode.Child(node)
		}

		return
	}
}

func (b *BuildLogs) buildPhaseSelectedTree(cfgNode *tree.Tree, cfg *configuration.Configuration) {
	p := b.conf.Fleet.PhaseStatus.Selected.Phase

	if p.GetPhaseScope() == phase.ScopeConfiguration {
		b.addPhases(cfgNode, cfg.Logs, cfg.Xpath, machineInd, false, p)

		return
	}

	for _, mp := range cfg.Machines.Pairs() {
		if mp.Value == nil || mp.Value.Logs == nil {
			continue
		}

		b.addMachineWithPhases(cfgNode, mp.Value, machineInd, p)
	}
}

func (b *BuildLogs) buildDefaultTree(cfgNode *tree.Tree, cfg *configuration.Configuration) {
	var machinePhases []phase.Phase

	flush := func() {
		if len(machinePhases) == 0 {
			return
		}

		for _, mp := range cfg.Machines.Pairs() {
			if mp.Value == nil || mp.Value.Logs == nil {
				continue
			}

			b.addMachineWithPhases(cfgNode, mp.Value, machineInd, machinePhases...)
		}

		machinePhases = nil
	}

	for _, pm := range phase.PhaseRegistry {
		if pm.Scope == phase.ScopeConfiguration {
			flush()
			b.addPhases(cfgNode, cfg.Logs, cfg.Xpath, machineInd, false, pm.Phase)
		} else {
			machinePhases = append(machinePhases, pm.Phase)
		}
	}

	flush()
}

func (b *BuildLogs) addMachineWithPhases(parent *tree.Tree, m *machine.Machine, indent int, allowed ...phase.Phase) {
	if m == nil || m.Logs == nil {
		return
	}

	node := b.entityNode(indent, b.conf.ColorScheme.Machine, m.Name, m.Logs, false)
	b.addPhases(node, m.Logs, m.Xpath, indent+treeStep, false, allowed...)

	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

func (b *BuildLogs) phaseLogsAndXpath(pm phase.PhaseMetadata, cfg *configuration.Configuration, m *machine.Machine) (*logs.Logs, xpath.Xpath) {
	if pm.Scope == phase.ScopeConfiguration {
		return cfg.Logs, cfg.Xpath
	}

	return m.Logs, m.Xpath
}

func (b *BuildLogs) isHideable(p phase.Phase) bool {
	return slices.Contains(hideablePhases, p)
}

func (b *BuildLogs) shouldHidePhase(p phase.Phase, phaseLog *phaselogs.PhaseLog) bool {
	tas := phaseLog.TimeAndState.Load()
	shouldHide := (!b.conf.Flags.Tui.ShowAllBuildLogs && b.isHideable(p)) || b.conf.Flags.Tui.ShowActiveOnly

	return shouldHide && tas.IsFinished() && tas.EndError == nil
}

func (b *BuildLogs) addPhases(parent *tree.Tree, logNode *logs.Logs, entityXpath xpath.Xpath, indent int, stopAtError bool, allowed ...phase.Phase) bool {
	if logNode == nil || logNode.PhaseLogs == nil {
		return false
	}

	for _, pair := range logNode.PhaseLogs.Pairs() {
		if !slices.Contains(allowed, pair.Key) {
			continue
		}

		if b.addPhase(parent, entityXpath, pair.Key, pair.Value, indent) && stopAtError {
			return true
		}
	}

	return false
}

func (b *BuildLogs) addPhase(parent *tree.Tree, entityXpath xpath.Xpath, p phase.Phase, phaseLog *phaselogs.PhaseLog, indent int) bool {
	if phaseLog == nil || b.shouldHidePhase(p, phaseLog) {
		return false
	}

	phaseXpath := entityXpath.NewXpathWithAppend(string(p))
	tas := phaseLog.TimeAndState

	icon := b.spinnerOrIcon(phaseXpath, string(b.conf.ColorScheme.Phase.Icon), tas.Load())
	durStyled, durWidth := b.durationText(b.conf.ColorScheme.Phase, tas)

	leftRaw := icon + strings.ToUpper(string(p))
	left := b.conf.ColorScheme.Phase.Color.Render(leftRaw)

	layoutIndent := indent
	if p.GetPhaseScope() == phase.ScopeConfiguration {
		layoutIndent -= 2
	}

	line := b.layoutLine(layoutIndent, left, durStyled, len(leftRaw), durWidth)

	phaseNode := tree.New().Root(b.conf.ColorScheme.Phase.Color.Render(line))
	hasError := b.addCommands(phaseNode, phaseLog, p, phaseXpath, indent)
	parent.Child(phaseNode)

	return hasError
}

func (b *BuildLogs) addCommands(phaseNode *tree.Tree, phaseLog *phaselogs.PhaseLog, p phase.Phase, phaseXpath xpath.Xpath, indent int) bool {
	hideable := b.isHideable(p)
	cmds := phaseLog.CommandLogs

	if cmds == nil {
		return phaseLog.TimeAndState.Load().EndError != nil
	}

	hasError := phaseLog.TimeAndState.Load().EndError != nil

	for idx, cmd := range cmds.Values() {
		if cmd == nil {
			continue
		}

		if !b.conf.Flags.Tui.ShowAllBuildLogs && hideable && idx != cmds.Length()-1 {
			continue
		}

		b.addCommand(phaseNode, cmd, idx, phaseXpath, indent)

		if cmd.TimeAndState != nil {
			if t := cmd.TimeAndState.Load(); t != nil && t.EndError != nil {
				hasError = true
			}
		}
	}

	return hasError
}

func (b *BuildLogs) addCommand(parent *tree.Tree, cmd *command.CommandLog, idx int, phaseXpath xpath.Xpath, indent int) {
	if cmd == nil || b.viewports == nil {
		return
	}

	cmdIndent := indent + treeStep

	label := cmd.Description
	if b.conf.Flags.Tui.ShowCommandsInLabels && len(cmd.Command) > 2 {
		label = cmd.Command
	}

	cmdXpath := phaseXpath.NewXpathWithAppend(label)
	tasCached := cmd.TimeAndState.Load()

	icon := b.spinnerOrIcon(cmdXpath, strconv.Itoa(idx+1), tasCached)
	durStyled, durWidth := b.durationText(b.conf.ColorScheme.Command, cmd.TimeAndState)

	labelWidth := cmdIndent + lipgloss.Width(icon) + durWidth
	labelVP := b.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("label"), label, labelWidth)

	if h := lipgloss.Height(labelVP); h > 1 {
		treeLine := "\n" + b.conf.ColorScheme.Tree.Enumerator.Render("│")
		icon += strings.Repeat(treeLine, h-1)
	}

	cmdNode := tree.New().Root(b.conf.ColorScheme.Command.Color.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, icon, b.conf.ColorScheme.Command.Color.Render(labelVP), durStyled),
	))

	output := cmd.StringForBuildLogs()

	outputXpath := cmdXpath.NewXpathWithAppend("output")
	if len(output) > 0 {
		cmdNode.Child(b.viewports.GetOrCreateViewport(outputXpath, output, cmdIndent+treeStep*2-1))
	} else {
		b.viewports.RemoveIfExistsViewport(outputXpath)
	}

	errXpath := cmdXpath.NewXpathWithAppend("error")
	if err := tasCached.EndError; err != nil {
		errMsg := "✗ Command failed: " + err.Error()
		cmdNode.Child(b.conf.ColorScheme.Error.Color.Render(
			b.viewports.GetOrCreateLabelViewport(errXpath, errMsg, cmdIndent+treeStep),
		))
	} else {
		b.viewports.RemoveIfExistsViewport(errXpath)
	}

	parent.Child(cmdNode)
}

func (b *BuildLogs) entityNode(indent int, style colorscheme.ColorSchemeLogEntity, name string, logNode *logs.Logs, isRoot bool) *tree.Tree {
	dur := 0.0
	if logNode != nil {
		dur = logNode.DurationAndErrorCache.Duration.Seconds()
	}

	leftRaw := fmt.Sprintf("%s %s", string(style.Icon), name)
	rightRaw := fmt.Sprintf(" (%.2fs)", dur)
	left := style.Color.Render(leftRaw)
	right := style.Color.Render(rightRaw)
	line := b.layoutLine(indent, left, right, len(leftRaw), len(rightRaw))

	t := tree.New().Root(line)
	if isRoot {
		t = t.Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(b.conf.ColorScheme.Tree.Enumerator).
			IndenterStyle(b.conf.ColorScheme.Tree.Enumerator)
	}

	return t
}

func (b *BuildLogs) layoutLine(indent int, left, right string, leftWidth, rightWidth int) string {
	leftWidth -= 2

	level := indent / treeStep
	available := b.viewports.ContentWidth() - indent - (timerIndent - level)
	pad := max(available-rightWidth-leftWidth, leftWidth)

	return left + strings.Repeat(" ", pad) + right
}

func (b *BuildLogs) spinnerOrIcon(xpath xpath.Xpath, icon string, tas *atomictimeandstate.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		return icon + " "
	}

	return b.spinners.View(xpath)
}

func (b *BuildLogs) durationText(style colorscheme.ColorSchemeLogEntity, tas *atomictimeandstate.AtomicTimeAndState) (string, int) {
	d, err := tas.DurationOrElapsedTime()
	if err != nil {
		return "", 0
	}

	text := fmt.Sprintf(" (%.2fs)", d.Seconds())

	return style.Color.Render(text), len(text)
}
