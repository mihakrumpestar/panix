package buildlogs

import (
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/style"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/internal/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/internal/pkg/xpath"
	"github.com/mihakrumpestar/panix/internal/tui/phasestatus"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

const (
	treeStep    = 3
	indentStep  = 2
	timerIndent = 4
	machineInd  = treeStep * indentStep
	phaseInd    = machineInd + treeStep

	iconCellWidth = 2
	maxSpaces     = 512
)

var (
	hideablePhasesSet = map[phase.Phase]struct{}{
		phase.Inspect: {},
		phase.Secrets: {},
	}

	upperPhaseNames = map[phase.Phase]string{
		phase.Inspect:   "INSPECT",
		phase.Build:     "BUILD",
		phase.Bootstrap: "BOOTSTRAP",
		phase.Transfer:  "TRANSFER",
		phase.Secrets:   "SECRETS",
		phase.Activate:  "ACTIVATE",
		phase.Rollback:  "ROLLBACK",
	}

	spaces = strings.Repeat(" ", maxSpaces)
)

type BuildLogs struct {
	conf        *config.Config
	statsTable  *statstable.StatsTable
	phaseStatus *phasestatus.PhaseStatus

	viewports *viewports.Viewports
	spinners  *spinners.Spinners

	styledTreeLine string
	contentWidth   int
}

func New(conf *config.Config, statsTable *statstable.StatsTable, phaseStatus *phasestatus.PhaseStatus) *BuildLogs {
	return &BuildLogs{
		conf:        conf,
		statsTable:  statsTable,
		phaseStatus: phaseStatus,
	}
}

func (b *BuildLogs) View(vp *viewports.Viewports, sp *spinners.Spinners) string {
	b.viewports = vp
	b.spinners = sp
	b.contentWidth = vp.ContentWidth()
	b.styledTreeLine = b.conf.ColorScheme.Tree.Enumerator.Render("│")

	var stringBuilder strings.Builder
	stringBuilder.WriteString(b.conf.ColorScheme.Header.Title.Render("=== Build Logs ===\n"))

	for _, fp := range b.conf.Fleet.Flakes.Pairs() {
		flake := fp.Value
		if flake == nil {
			continue
		}

		flakeNode := b.entityNode(0, b.conf.ColorScheme.Flake, flake.Name, flake.Logs, true)

		for _, cp := range flake.Configurations.Pairs() {
			cfg := cp.Value
			if cfg == nil {
				continue
			}

			cfgNode := b.entityNode(treeStep, b.conf.ColorScheme.Configuration, cfg.Name, cfg.Logs, false)
			b.buildConfigTree(cfgNode, cfg)

			if cfgNode.Length() > 0 {
				flakeNode.Child(cfgNode)
			}
		}

		if flakeNode.Length() > 0 {
			flakeNode.RenderTo(&stringBuilder)
		}
	}

	return stringBuilder.String()
}

func (b *BuildLogs) buildConfigTree(cfgNode *tree.Node, cfg *configuration.Configuration) {
	switch {
	case b.statsTable.SelectedIndex() >= 0:
		b.buildMachineSelectedTree(cfgNode, cfg)
	case b.phaseStatus.Selected.Index >= 0:
		b.buildPhaseSelectedTree(cfgNode, cfg)
	default:
		b.buildDefaultTree(cfgNode, cfg)
	}
}

func (b *BuildLogs) buildMachineSelectedTree(cfgNode *tree.Node, cfg *configuration.Configuration) {
	for _, mp := range cfg.Machines.Pairs() {
		machine := mp.Value
		if machine == nil || machine.Xpath != b.statsTable.SelectedXpath() {
			continue
		}

		if machine.Logs == nil {
			return
		}

		node := b.entityNode(machineInd, b.conf.ColorScheme.Machine, machine.Name, machine.Logs, false)

		errored := b.addPhases(node, machine.Logs, machine.Xpath, phaseInd, true, phase.Inspect)
		if !errored {
			for _, pm := range phase.PhaseRegistry[1:] {
				logs_, xp := b.phaseLogsAndXpath(pm, cfg, machine)
				if b.addPhases(node, logs_, xp, phaseInd, true, pm.Phase) {
					break
				}
			}
		}

		if node.Length() > 0 {
			cfgNode.Child(node)
		}

		return
	}
}

func (b *BuildLogs) buildPhaseSelectedTree(cfgNode *tree.Node, cfg *configuration.Configuration) {
	phaseI := phase.Phase(b.phaseStatus.Selected.Phase)

	if phaseI.GetPhaseScope() == phase.ScopeConfiguration {
		b.addPhases(cfgNode, cfg.Logs, cfg.Xpath, machineInd, false, phaseI)

		return
	}

	for _, mp := range cfg.Machines.Pairs() {
		if mp.Value == nil || mp.Value.Logs == nil {
			continue
		}

		b.addMachineWithPhases(cfgNode, mp.Value, machineInd, phaseI)
	}
}

func (b *BuildLogs) buildDefaultTree(cfgNode *tree.Node, cfg *configuration.Configuration) {
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

func (b *BuildLogs) addMachineWithPhases(parent *tree.Node, machine *machine.Machine, indent int, allowed ...phase.Phase) {
	if machine == nil || machine.Logs == nil {
		return
	}

	node := b.entityNode(indent, b.conf.ColorScheme.Machine, machine.Name, machine.Logs, false)
	b.addPhases(node, machine.Logs, machine.Xpath, indent+treeStep, false, allowed...)

	if node.Length() > 0 {
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
	_, ok := hideablePhasesSet[p]

	return ok
}

func (b *BuildLogs) shouldHidePhase(p phase.Phase, phaseLog *phaselogs.PhaseLog) bool {
	tas := phaseLog.TimeAndState.Load()
	shouldHide := (!b.conf.Flags.Tui.ShowAllBuildLogs && b.isHideable(p)) || b.conf.Flags.Tui.ShowActiveOnly

	return shouldHide && tas.IsFinished() && tas.EndError == nil
}

func makeAllowedSet(allowed []phase.Phase) map[phase.Phase]struct{} {
	s := make(map[phase.Phase]struct{}, len(allowed))
	for _, p := range allowed {
		s[p] = struct{}{}
	}

	return s
}

func (b *BuildLogs) addPhases(
	parent *tree.Node,
	logNode *logs.Logs,
	entityXpath xpath.Xpath,
	indent int, stopAtError bool,
	allowed ...phase.Phase,
) bool {
	if logNode == nil || logNode.PhaseLogs == nil {
		return false
	}

	if len(allowed) == 1 {
		return b.addPhasesSingle(parent, logNode, entityXpath, indent, stopAtError, allowed[0])
	}

	return b.addPhasesMulti(parent, logNode, entityXpath, indent, stopAtError, allowed)
}

func (b *BuildLogs) addPhasesSingle(
	parent *tree.Node,
	logNode *logs.Logs,
	entityXpath xpath.Xpath,
	indent int, stopAtError bool,
	allowedPhase phase.Phase,
) bool {
	for _, pair := range logNode.PhaseLogs.Pairs() {
		if pair.Key != allowedPhase {
			continue
		}

		if b.addPhase(parent, entityXpath, pair.Key, pair.Value, indent) && stopAtError {
			return true
		}
	}

	return false
}

func (b *BuildLogs) addPhasesMulti(
	parent *tree.Node,
	logNode *logs.Logs,
	entityXpath xpath.Xpath,
	indent int, stopAtError bool,
	allowed []phase.Phase,
) bool {
	allowedSet := makeAllowedSet(allowed)

	for _, pair := range logNode.PhaseLogs.Pairs() {
		_, ok := allowedSet[pair.Key]
		if !ok {
			continue
		}

		if b.addPhase(parent, entityXpath, pair.Key, pair.Value, indent) && stopAtError {
			return true
		}
	}

	return false
}

func (b *BuildLogs) addPhase(parent *tree.Node, entityXpath xpath.Xpath, phaseI phase.Phase, phaseLog *phaselogs.PhaseLog, indent int) bool {
	if phaseLog == nil || b.shouldHidePhase(phaseI, phaseLog) {
		return false
	}

	phaseXpath := entityXpath.NewXpathWithAppend(phaseI.String())
	tas := phaseLog.TimeAndState

	tasLoaded := tas.Load()
	icon := b.spinnerOrIcon(phaseXpath, b.conf.ColorScheme.Phase.Icon, tasLoaded)
	durStyled, durWidth := b.durationText(b.conf.ColorScheme.Phase.Color, tas)

	upperName := upperPhaseNames[phaseI]
	if upperName == "" {
		upperName = strings.ToUpper(phaseI.String())
	}

	leftRaw := icon + upperName
	leftWidth := style.CellWidth(icon) + len(upperName)
	left := b.conf.ColorScheme.Phase.Color.Render(leftRaw)

	layoutIndent := indent
	if phaseI.GetPhaseScope() == phase.ScopeConfiguration {
		layoutIndent -= 2
	}

	line := b.layoutLine(layoutIndent, left, durStyled, leftWidth, durWidth)

	phaseNode := tree.New().Root(line)
	hasError := b.addCommands(phaseNode, phaseLog, phaseI, phaseXpath, indent)
	parent.Child(phaseNode)

	return hasError
}

func (b *BuildLogs) addCommands(phaseNode *tree.Node, phaseLog *phaselogs.PhaseLog, p phase.Phase, phaseXpath xpath.Xpath, indent int) bool {
	hideable := b.isHideable(p)
	cmds := phaseLog.CommandLogs

	tasLoaded := phaseLog.TimeAndState.Load()
	if cmds == nil {
		return tasLoaded.EndError != nil
	}

	hasError := tasLoaded.EndError != nil

	cmdValues := cmds.Values()
	cmdLen := len(cmdValues)
	lastIdx := cmdLen - 1

	for idx, cmd := range cmdValues {
		if cmd == nil {
			continue
		}

		if !b.conf.Flags.Tui.ShowAllBuildLogs && hideable && idx != lastIdx {
			continue
		}

		b.addCommand(phaseNode, cmd, idx, phaseXpath, indent)

		if cmd.TimeAndState != nil {
			t := cmd.TimeAndState.Load()
			if t != nil && t.EndError != nil {
				hasError = true
			}
		}
	}

	return hasError
}

func (b *BuildLogs) addCommand(parent *tree.Node, cmd *command.CommandLog, idx int, phaseXpath xpath.Xpath, indent int) {
	if cmd == nil || b.viewports == nil {
		return
	}

	cmdIndent := indent + treeStep
	cmdXpath := phaseXpath.NewXpathWithAppend(cmd.Description)

	label := cmd.Description
	if b.conf.Flags.Tui.ShowCommandsInLabels && len(cmd.Command) > 2 {
		label = cmd.Command
	}

	var labelShowsCommands uint64
	if b.conf.Flags.Tui.ShowCommandsInLabels {
		labelShowsCommands = 1
	}

	tasCached := cmd.TimeAndState.Load()

	icon := b.spinnerOrIcon(cmdXpath, strconv.Itoa(idx+1), tasCached)
	icon = b.conf.ColorScheme.Command.Color.Render(icon)

	durStyled, durWidth := b.durationText(b.conf.ColorScheme.Command.Color, cmd.TimeAndState)

	iconWidth := style.CellWidth(icon)
	labelWidth := cmdIndent + iconWidth + durWidth
	labelXpath := cmdXpath.NewXpathWithAppend("label")
	labelVP := b.viewports.GetOrCreateLabelViewport(labelXpath, label, labelShowsCommands, labelWidth)

	labelLineCount := style.CountLines(labelVP)
	if labelLineCount > 1 {
		treeLine := "\n" + b.styledTreeLine
		icon += strings.Repeat(treeLine, labelLineCount-1)
	}

	styledLabel := b.conf.ColorScheme.Command.Color.Render(labelVP)

	cmdContent := style.JoinHorizontal(style.Top, icon, styledLabel, durStyled)
	cmdNode := tree.New().Root(cmdContent)

	b.addCommandChildren(cmdNode, cmd, cmdXpath, tasCached, cmdIndent)

	parent.Child(cmdNode)
}

func (b *BuildLogs) addCommandChildren(
	cmdNode *tree.Node, cmd *command.CommandLog,
	cmdXpath xpath.Xpath, tasCached *atomictimeandstate.TimeAndState,
	cmdIndent int,
) {
	output := cmd.Output
	outputXpath := cmdXpath.NewXpathWithAppend("output")

	if output.Len() > 0 {
		cmdNode.ChildString(b.viewports.GetOrCreateViewportVersioned(outputXpath, output, cmdIndent+treeStep))
	}

	errXpath := cmdXpath.NewXpathWithAppend("error")

	err := tasCached.EndError
	if err != nil {
		errMsg := b.conf.ColorScheme.Chars.ErrorIcon + " Command failed: " + err.Error()
		cmdNode.ChildString(b.conf.ColorScheme.Error.Color.Render(
			b.viewports.GetOrCreateLabelViewport(errXpath, errMsg, 0, cmdIndent+treeStep),
		))
	}
}

func (b *BuildLogs) entityNode(indent int, style colorscheme.ColorSchemeLogEntity, name string, logNode *logs.Logs, isRoot bool) *tree.Node {
	dur := 0.0
	if logNode != nil {
		dur = logNode.DurationAndErrorCache.Duration.Seconds()
	}

	ansi := b.styleForEntity(style)

	leftRaw := style.Icon + " " + name
	rightRaw := formatDuration(dur)
	left := ansi.Render(leftRaw)
	right := ansi.Render(rightRaw)

	leftWidth := iconCellWidth + 1 + len(name)
	rightWidth := len(rightRaw)

	line := b.layoutLine(indent, left, right, leftWidth, rightWidth)

	treeI := tree.New().Root(line)
	if isRoot {
		treeI = treeI.EnumeratorStyle(b.conf.ColorScheme.Tree.Enumerator).
			IndenterStyle(b.conf.ColorScheme.Tree.Enumerator)
	}

	return treeI
}

func (b *BuildLogs) styleForEntity(entity colorscheme.ColorSchemeLogEntity) style.Style {
	return entity.Color
}

func (b *BuildLogs) layoutLine(indent int, left, right string, leftWidth, rightWidth int) string {
	level := indent / treeStep
	available := b.contentWidth - indent - (timerIndent - level)
	pad := max(available-rightWidth-leftWidth, leftWidth)

	if pad <= maxSpaces {
		return left + spaces[:pad] + right
	}

	return left + strings.Repeat(" ", pad) + right
}

func (b *BuildLogs) spinnerOrIcon(xpathVal xpath.Xpath, icon string, tas *atomictimeandstate.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		return icon + " "
	}

	return b.spinners.View(xpathVal)
}

func (b *BuildLogs) durationText(sty style.Style, tas *atomictimeandstate.AtomicTimeAndState) (string, int) {
	d, err := tas.DurationOrElapsedTime()
	if err != nil {
		return "", 0
	}

	text := formatDuration(d.Seconds())

	return sty.Render(text), len(text)
}

func formatDuration(secs float64) string {
	var buf [32]byte

	result := strconv.AppendFloat(buf[:0], secs, 'f', 2, 64) //nolint:mnd

	return " (" + string(result) + "s)"
}
