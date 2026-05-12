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
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

const (
	treeStep    = 3
	indentStep  = 2
	timerIndent = 4
	machineInd  = treeStep * indentStep
	phaseInd    = machineInd + treeStep

	maxSpaces = 512
)

var (
	hideablePhasesSet = map[phase.Phase]struct{}{
		phase.Inspect: {},
		phase.Secrets: {},
	}

	upperPhaseNames = map[phase.Phase][]byte{
		phase.Inspect:   []byte("INSPECT"),
		phase.Build:     []byte("BUILD"),
		phase.Bootstrap: []byte("BOOTSTRAP"),
		phase.Transfer:  []byte("TRANSFER"),
		phase.Secrets:   []byte("SECRETS"),
		phase.Activate:  []byte("ACTIVATE"),
		phase.Rollback:  []byte("ROLLBACK"),
	}

	spacesBytes = []byte(strings.Repeat(" ", maxSpaces))

	headerTitle = []byte("=== Build Logs ===")
)

type BuildLogs struct {
	conf        *config.Config
	statsTable  *statstable.StatsTable
	phaseStatus *phaseflow.PhaseFlow

	viewports *viewports.Viewports
	spinners  *spinners.Spinners

	styledTreeLine []byte
	contentWidth   int
	content        *buffer.LinesBuf
	tree           *tree.Node
	nodeBufs       []*buffer.LinesBuf

	cmdIconBuf  *buffer.LinesBuf
	cmdLabelBuf *buffer.LinesBuf
	cmdDurBuf   *buffer.LinesBuf
	errBuf      *buffer.LinesBuf
	durLineBuf  *buffer.LineBuf
	iconBuf     *buffer.LineBuf
}

func New(conf *config.Config, statsTable *statstable.StatsTable, phaseStatus *phaseflow.PhaseFlow) *BuildLogs {
	return &BuildLogs{
		conf:        conf,
		statsTable:  statsTable,
		phaseStatus: phaseStatus,
		content:     buffer.NewLinesBuf(),
		tree:        tree.NewTree(conf.ColorScheme.Tree.Enumerator),
		cmdIconBuf:  buffer.NewLinesBuf(),
		cmdLabelBuf: buffer.NewLinesBuf(),
		cmdDurBuf:   buffer.NewLinesBuf(),
		errBuf:      buffer.NewLinesBuf(),
		durLineBuf:  buffer.NewLineBuf(),
		iconBuf:     buffer.NewLineBuf(),
	}
}

// Render renders the build logs tree and returns the output buffer.
func (b *BuildLogs) Render(vp *viewports.Viewports, sp *spinners.Spinners) *buffer.LinesBuf {
	for _, nb := range b.nodeBufs {
		nb.Release()
	}

	b.nodeBufs = b.nodeBufs[:0]

	b.viewports = vp
	b.spinners = sp
	b.contentWidth = vp.ContentWidth()
	b.styledTreeLine = b.conf.ColorScheme.Tree.Enumerator.RenderLine([]byte("│"))

	b.content.Reset()
	b.conf.ColorScheme.Header.Title.RenderInto(b.content, [][]byte{headerTitle})
	b.content.EmptyLine()

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

			if cfgNode.Len() > 0 {
				flakeNode.Child(cfgNode)
			}
		}

		if flakeNode.Len() > 0 {
			flakeNode.RenderInto(b.content)
		}
	}

	return b.content
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

		if node.Len() > 0 {
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

	if node.Len() > 0 {
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
	durStyled, durWidth := b.durationBytes(b.conf.ColorScheme.Phase.Color, tas)

	upperName := upperPhaseNames[phaseI]
	if upperName == nil {
		upperName = []byte(strings.ToUpper(phaseI.String()))
	}

	b.iconBuf.Reset()
	b.iconBuf.Write(icon)
	b.iconBuf.Write(upperName)
	leftRaw := b.iconBuf.Bytes()

	leftWidth := style.CellWidth(icon) + len(upperName)

	layoutIndent := indent
	if phaseI.GetPhaseScope() == phase.ScopeConfiguration {
		layoutIndent -= 2
	}

	line := b.layoutLineStyled(layoutIndent, b.conf.ColorScheme.Phase.Color, leftRaw, durStyled, leftWidth, durWidth)

	phaseNode := b.tree.NewNode(line)
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

	var label string
	if b.conf.Flags.Tui.ShowCommandsInLabels && cmd.Command != nil && len(cmd.Command.Bytes()) > 2 {
		label = string(cmd.Command.Bytes())
	} else {
		label = cmd.Description
	}

	var labelShowsCommands uint64
	if b.conf.Flags.Tui.ShowCommandsInLabels {
		labelShowsCommands = 1
	}

	tasCached := cmd.TimeAndState.Load()

	b.iconBuf.Reset()
	b.iconBuf.WriteString(strconv.Itoa(idx + 1))
	icon := b.spinnerOrIcon(cmdXpath, b.iconBuf.Bytes(), tasCached)

	b.cmdIconBuf.Reset()
	b.conf.ColorScheme.Command.Color.RenderLineInto(b.cmdIconBuf, icon)

	durStyled, durWidth := b.durationBytes(b.conf.ColorScheme.Command.Color, cmd.TimeAndState)

	iconWidth := style.CellWidth(b.cmdIconBuf.Line(0))
	labelWidth := cmdIndent + iconWidth + durWidth
	labelXpath := cmdXpath.NewXpathWithAppend("label")
	labelResult := b.viewports.RenderLabelViewport(labelXpath, [][]byte{[]byte(label)}, labelShowsCommands, labelWidth)

	b.cmdLabelBuf.Reset()
	b.conf.ColorScheme.Command.Color.RenderLineInto(b.cmdLabelBuf, labelResult.Line(0))

	b.cmdDurBuf.Reset()
	b.conf.ColorScheme.Command.Color.RenderLineInto(b.cmdDurBuf, durStyled)

	joinBuf := b.acquireNodeBuf()
	style.JoinHorizontalBufs(joinBuf, style.Top, b.cmdIconBuf, b.cmdLabelBuf, b.cmdDurBuf)

	cmdNode := b.tree.NewNode(joinBuf)

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
		outResult := b.viewports.RenderViewportVersioned(outputXpath, output.Lines(), output.Version(), cmdIndent+treeStep)
		outBuf := b.acquireNodeBuf()
		outBuf.AppendFrom(outResult)
		cmdNode.ChildContent(outBuf)
	}

	errXpath := cmdXpath.NewXpathWithAppend("error")

	err := tasCached.EndError
	if err != nil {
		errMsg := append(
			append(
				append([]byte{}, b.conf.ColorScheme.Chars.ErrorIcon...),
				" Command failed: "...,
			),
			err.Error()...,
		)
		errResult := b.viewports.RenderLabelViewport(errXpath, [][]byte{errMsg}, 0, cmdIndent+treeStep)
		b.errBuf.Reset()
		b.conf.ColorScheme.Error.Color.RenderInto(b.errBuf, errResult.Lines())
		errNodeBuf := b.acquireNodeBuf()
		errNodeBuf.AppendFrom(b.errBuf)
		cmdNode.ChildContent(errNodeBuf)
	}
}

func (b *BuildLogs) entityNode(indent int, entity colorscheme.ColorSchemeLogEntity, name string, logNode *logs.Logs, _ bool) *tree.Node {
	dur := 0.0
	if logNode != nil {
		dur = logNode.DurationAndErrorCache.Duration.Seconds()
	}

	b.iconBuf.Reset()
	b.iconBuf.Write(entity.Icon)
	b.iconBuf.WriteByte(' ')
	b.iconBuf.WriteString(name)
	leftRaw := b.iconBuf.Bytes()

	rightRaw := b.formatDuration(dur)

	leftWidth := len(entity.Icon) + 1 + len(name)
	rightWidth := len(rightRaw)

	line := b.layoutLineStyled(indent, entity.Color, leftRaw, rightRaw, leftWidth, rightWidth)

	node := b.tree.NewNode(line)

	return node
}

// layoutLineStyled renders styled left + pad + styled right into a node buffer.
// Zero-allocation for the common color-only case.
func (b *BuildLogs) layoutLineStyled(indent int, sty style.Style, leftRaw, rightRaw []byte, leftWidth, rightWidth int) *buffer.LinesBuf {
	level := indent / treeStep
	available := b.contentWidth - indent - (timerIndent - level)
	pad := max(available-rightWidth-leftWidth, leftWidth)

	var padBytes []byte
	if pad <= maxSpaces {
		padBytes = spacesBytes[:pad]
	} else {
		padBytes = []byte(strings.Repeat(" ", pad))
	}

	lb := b.acquireNodeBuf()

	// For color-only styles: prefix + leftRaw + reset + pad + prefix + rightRaw + reset
	// This is exactly what WriteLine3 gives us when we pass styled bytes.
	// We build the styled bytes via RenderLine which returns a new []byte.
	// For the no-layout case (common), we can avoid allocs by writing directly.
	if !sty.HasLayoutProperties() {
		prefix := sty.StylePrefix()
		reset := style.ANSIReset()

		if len(prefix) == 0 {
			lb.WriteLine3(leftRaw, padBytes, rightRaw)
		} else {
			// Write prefix + leftRaw + reset + padBytes + prefix + rightRaw + reset
			// as one line using AppendToLine
			lb.EmptyLine()
			lb.AppendToLine(prefix, leftRaw, reset, padBytes, prefix, rightRaw, reset)
		}
	} else {
		left := sty.RenderLine(leftRaw)
		right := sty.RenderLine(rightRaw)
		lb.WriteLine3(left, padBytes, right)
	}

	return lb
}

func (b *BuildLogs) acquireNodeBuf() *buffer.LinesBuf {
	lb := buffer.NewLinesBuf()
	b.nodeBufs = append(b.nodeBufs, lb)

	return lb
}

func (b *BuildLogs) spinnerOrIcon(xpathVal xpath.Xpath, icon []byte, tas *atomictimeandstate.TimeAndState) []byte {
	if !tas.HasStarted() {
		return nil
	}

	if tas.IsFinished() {
		b.iconBuf.Reset()
		b.iconBuf.Write(icon)
		b.iconBuf.WriteByte(' ')

		return b.iconBuf.Bytes()
	}

	frame := b.spinners.Render(xpathVal)
	b.iconBuf.Reset()
	b.iconBuf.Write(frame)
	b.iconBuf.WriteByte(' ')

	return b.iconBuf.Bytes()
}

func (b *BuildLogs) durationBytes(sty style.Style, tas *atomictimeandstate.AtomicTimeAndState) ([]byte, int) {
	d, err := tas.DurationOrElapsedTime()
	if err != nil {
		return nil, 0
	}

	text := b.formatDuration(d.Seconds())

	return text, len(text)
}

func (b *BuildLogs) formatDuration(secs float64) []byte {
	b.durLineBuf.Reset()
	b.durLineBuf.WriteByte(' ')
	b.durLineBuf.WriteByte('(')

	var tmp [32]byte

	result := strconv.AppendFloat(tmp[:0], secs, 'f', 2, 64) //nolint:mnd
	b.durLineBuf.Write(result)
	b.durLineBuf.WriteString("s)")

	return b.durLineBuf.Bytes()
}
