package buildlogs

import (
	"strconv"
	"strings"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/style"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

const (
	// TreeStep is the indent width per depth level.
	TreeStep          = 3
	timerIndent       = 4
	timerLevelPhase   = 3
	timerLevelCommand = 4

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

// reuseBuf resets old if non-nil, otherwise returns a new LinesBuf.
// Used by tree calculate callbacks to preserve buffer capacity across GC cycles.
func reuseBuf(old *buffer.LinesBuf) *buffer.LinesBuf {
	if old != nil {
		old.Reset()

		return old
	}

	return buffer.NewLinesBuf()
}

type BuildLogs struct {
	conf        *config.Config
	statsTable  *statstable.StatsTable
	phaseStatus *phaseflow.PhaseFlow

	viewports *viewports.Viewports
	spinners  *spinners.Spinners

	styledTreeLine []byte
	contentWidth   int
	lastWidth      int
	widthOffset    uint64

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
		cmdIconBuf:  buffer.NewLinesBuf(),
		cmdLabelBuf: buffer.NewLinesBuf(),
		cmdDurBuf:   buffer.NewLinesBuf(),
		errBuf:      buffer.NewLinesBuf(),
		durLineBuf:  buffer.NewLineBuf(),
		iconBuf:     buffer.NewLineBuf(),
	}
}

// RenderInto renders the build logs tree directly into target, avoiding
// an intermediate buffer copy. Widths set by the tree renderer (via
// writeLines/renderLeaf) are set directly on target, so the viewport can
// adopt them via adoptCachedLineWidths, eliminating redundant CellWidth
// scans in padNewLine.
func (b *BuildLogs) RenderInto(
	target *buffer.LinesBuf,
	treeNode *tree.Node,
	viewports *viewports.Viewports,
	spinners *spinners.Spinners,
) {
	b.viewports = viewports
	b.spinners = spinners
	b.contentWidth = viewports.ContentWidth()
	b.styledTreeLine = b.conf.ColorScheme.Tree.Enumerator.RenderLine([]byte("│"))

	widthChanged := b.contentWidth != b.lastWidth
	b.lastWidth = b.contentWidth

	b.conf.ColorScheme.Header.Title.RenderLineInto(target, headerTitle)
	target.EmptyLine()

	treeNode.BeginFrame()

	b.widthOffset = 0

	if widthChanged {
		const widthShift = 32

		b.widthOffset = uint64(b.contentWidth) << widthShift //nolint:gosec // G115: contentWidth is always positive
	}

	b.conf.Fleet.Flakes.ForEach(func(_ string, flake *flake.Flake) bool {
		if flake == nil {
			return true
		}

		flakeVersion := b.entityVersion(flake.Logs)
		flakeNode := treeNode.Child(flake.Xpath, flakeVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
			return b.entityNodeContent(depthWidth, b.conf.ColorScheme.Flake, flake.Name.String(), flake.Logs, old)
		})

		b.buildInstallablesTree(flakeNode, flake)

		return true
	})

	treeNode.WriteRenderTo(target)
}

// buildInstallablesTree renders the two-level installables map (type → name → installable)
// as nested tree nodes under flakeNode.
func (b *BuildLogs) buildInstallablesTree(flakeNode *tree.Node, flake *flake.Flake) {
	flake.Installables.ForEach(func(typeKey string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installable.Installable]) bool {
		if attrMap == nil {
			return true
		}

		// Pre-compute max duration across all installables of this type
		// so the type grouping node can display it.
		var maxTypeDuration time.Duration

		typeRunning := false

		attrMap.ForEach(func(_ string, inst *installable.Installable) bool {
			if inst != nil && inst.Logs != nil {
				dur := inst.Logs.TAS.DurationCache
				if dur > maxTypeDuration {
					maxTypeDuration = dur
				}

				if !inst.Logs.TAS.IsFinished() {
					typeRunning = true
				}
			}

			return true
		})

		// Create output type grouping node (e.g., "nixosConfigurations").
		// tree.Node.Child is idempotent: same xpath returns the same node,
		// so multiple outputs of the same type share one type node.
		// Version is coarse (1/10th second) to avoid re-rendering every frame.
		outputTypeXpath := flake.Xpath.NewXpathWithAppend(typeKey)

		typeVersion := b.widthOffset
		if typeRunning {
			typeVersion += b.spinners.Generation()
		}

		typeNode := flakeNode.Child(outputTypeXpath, typeVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
			return b.entityNodeContentWithDuration(depthWidth, b.conf.ColorScheme.Installable, typeKey, maxTypeDuration, old)
		})

		attrMap.ForEach(func(nameKey string, installable *installable.Installable) bool {
			if installable == nil {
				return true
			}

			// Create output name node (e.g., "server1").
			outVersion := b.entityVersion(installable.Logs)
			outNode := typeNode.Child(installable.Xpath, outVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
				return b.entityNodeContent(depthWidth, b.conf.ColorScheme.Installable, installable.Name.String(), installable.Logs, old)
			})
			b.buildOutputTree(outNode, installable)

			return true
		})

		return true
	})
}

func (b *BuildLogs) buildOutputTree(outNode *tree.Node, installable *installable.Installable) {
	switch {
	case b.statsTable.SelectedIndex() >= 0:
		b.buildMachineSelectedTree(outNode, installable)
	case b.phaseStatus.Selected.Index >= 0:
		b.buildPhaseSelectedTree(outNode, installable)
	default:
		b.buildDefaultTree(outNode, installable)
	}
}

func (b *BuildLogs) buildMachineSelectedTree(outNode *tree.Node, installable *installable.Installable) {
	for _, mp := range installable.Machines.Pairs() {
		machine := mp.Value
		if machine == nil || machine.Xpath != b.statsTable.SelectedXpath() {
			continue
		}

		if machine.Logs == nil {
			return
		}

		entityVersion := b.entityVersion(machine.Logs)
		machineNode := outNode.Child(machine.Xpath, entityVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
			return b.entityNodeContent(depthWidth, b.conf.ColorScheme.Machine, machine.Name.String(), machine.Logs, old)
		})

		errored := b.addPhases(machineNode, machine.Logs, machine.PhaseXpaths, true, entityVersion, phase.Inspect)
		if !errored {
			for _, pm := range phase.PhaseRegistry[1:] {
				logs_, xps := b.phaseLogsAndXpath(pm, installable, machine)
				if b.addPhases(machineNode, logs_, xps, true, entityVersion, pm.Phase) {
					break
				}
			}
		}

		return
	}
}

func (b *BuildLogs) buildPhaseSelectedTree(outNode *tree.Node, installable *installable.Installable) {
	phaseI := phase.Phase(b.phaseStatus.Selected.Phase)
	outVersion := b.entityVersion(installable.Logs)

	if phaseI.GetPhaseScope() == phase.ScopeInstallable {
		b.addPhases(outNode, installable.Logs, installable.PhaseXpaths, false, outVersion, phaseI)

		return
	}

	installable.Machines.ForEach(func(_ string, machine *machine.Machine) bool {
		if machine == nil || machine.Logs == nil {
			return true
		}

		b.addMachineWithPhases(outNode, machine, map[phase.Phase]struct{}{phaseI: {}})

		return true
	})
}

func (b *BuildLogs) buildDefaultTree(outNode *tree.Node, installable *installable.Installable) {
	outVersion := b.entityVersion(installable.Logs)

	var machinePhases []phase.Phase

	flush := func() {
		if len(machinePhases) == 0 {
			return
		}

		allowedSet := makeAllowedSet(machinePhases)

		installable.Machines.ForEach(func(_ string, machine *machine.Machine) bool {
			if machine == nil || machine.Logs == nil {
				return true
			}

			b.addMachineWithPhases(outNode, machine, allowedSet)

			return true
		})

		machinePhases = nil
	}

	for _, phaseMetadata := range phase.PhaseRegistry {
		if phaseMetadata.Scope == phase.ScopeInstallable {
			flush()
			b.addPhases(outNode, installable.Logs, installable.PhaseXpaths, false, outVersion, phaseMetadata.Phase)
		} else {
			machinePhases = append(machinePhases, phaseMetadata.Phase)
		}
	}

	flush()
}

func (b *BuildLogs) addMachineWithPhases(parent *tree.Node, machine *machine.Machine, allowedSet map[phase.Phase]struct{}) {
	if machine == nil || machine.Logs == nil {
		return
	}

	if !b.hasVisiblePhases(machine.Logs, allowedSet) {
		return
	}

	entityVersion := b.entityVersion(machine.Logs)
	machineNode := parent.Child(machine.Xpath, entityVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
		return b.entityNodeContent(depthWidth, b.conf.ColorScheme.Machine, machine.Name.String(), machine.Logs, old)
	})

	b.addPhasesMulti(machineNode, machine.Logs, machine.PhaseXpaths, false, entityVersion, allowedSet)
}

func (b *BuildLogs) hasVisiblePhases(logNode *logs.Logs, allowedSet map[phase.Phase]struct{}) bool {
	if logNode == nil || logNode.PhaseLogs == nil {
		return false
	}

	hasVisible := false

	logNode.PhaseLogs.ForEach(func(phaseKey phase.Phase, phaseValue *phaselogs.PhaseLog) bool {
		_, ok := allowedSet[phaseKey]
		if !ok {
			return true
		}

		if !b.shouldHidePhase(phaseKey, phaseValue) {
			hasVisible = true

			return false
		}

		return true
	})

	return hasVisible
}

func (b *BuildLogs) phaseLogsAndXpath(
	pm phase.PhaseMetadata,
	installable *installable.Installable,
	mach *machine.Machine,
) (*logs.Logs, map[phase.Phase]xpath.Xpath) {
	if pm.Scope == phase.ScopeInstallable {
		return installable.Logs, installable.PhaseXpaths
	}

	return mach.Logs, mach.PhaseXpaths
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
	phaseXpaths map[phase.Phase]xpath.Xpath,
	stopAtError bool,
	entityVersion uint64,
	allowed ...phase.Phase,
) bool {
	if logNode == nil || logNode.PhaseLogs == nil {
		return false
	}

	if len(allowed) == 1 {
		return b.addPhasesSingle(parent, logNode, phaseXpaths, stopAtError, entityVersion, allowed[0])
	}

	return b.addPhasesMulti(parent, logNode, phaseXpaths, stopAtError, entityVersion, makeAllowedSet(allowed))
}

func (b *BuildLogs) addPhasesSingle(
	parent *tree.Node,
	logNode *logs.Logs,
	phaseXpaths map[phase.Phase]xpath.Xpath,
	stopAtError bool,
	entityVersion uint64,
	allowedPhase phase.Phase,
) bool {
	stopped := false

	logNode.PhaseLogs.ForEach(func(phaseKey phase.Phase, phaseValue *phaselogs.PhaseLog) bool {
		if phaseKey != allowedPhase {
			return true
		}

		phaseXpath := phaseXpaths[phaseKey]

		if b.addPhase(parent, phaseXpath, phaseKey, phaseValue, entityVersion) && stopAtError {
			stopped = true

			return false
		}

		return true
	})

	return stopped
}

func (b *BuildLogs) addPhasesMulti(
	parent *tree.Node,
	logNode *logs.Logs,
	phaseXpaths map[phase.Phase]xpath.Xpath,
	stopAtError bool,
	entityVersion uint64,
	allowedSet map[phase.Phase]struct{},
) bool {
	for _, phaseMetadata := range phase.PhaseRegistry {
		_, ok := allowedSet[phaseMetadata.Phase]
		if !ok {
			continue
		}

		phaseLog, ok := logNode.PhaseLogs.Get(phaseMetadata.Phase)
		if !ok || phaseLog == nil {
			continue
		}

		phaseXpath := phaseXpaths[phaseMetadata.Phase]

		if b.addPhase(parent, phaseXpath, phaseMetadata.Phase, phaseLog, entityVersion) && stopAtError {
			return true
		}
	}

	return false
}

func (b *BuildLogs) addPhase(
	parent *tree.Node,
	phaseXpath xpath.Xpath,
	phaseI phase.Phase,
	phaseLog *phaselogs.PhaseLog,
	entityVersion uint64,
) bool {
	if phaseLog == nil || b.shouldHidePhase(phaseI, phaseLog) {
		return false
	}

	upperName := upperPhaseNames[phaseI]
	if upperName == nil {
		upperName = []byte(strings.ToUpper(phaseI.String()))
	}

	displayVersion := entityVersion + phaseLog.TimeAndState.StateVersion()

	phaseNode := parent.Child(phaseXpath, displayVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
		tas := phaseLog.TimeAndState
		tasLoaded := tas.Load()
		icon := b.spinnerOrIcon(phaseXpath, b.conf.ColorScheme.Phase.Icon, tasLoaded)
		durStyled, durWidth := b.durationBytes(tas)

		b.iconBuf.Reset()
		b.iconBuf.Write(icon)
		b.iconBuf.Write(upperName)
		leftRaw := b.iconBuf.Bytes()

		leftWidth := style.CellWidth(icon) + len(upperName)

		return b.layoutLineStyled(depthWidth, timerLevelPhase, b.conf.ColorScheme.Phase.Color, leftRaw, durStyled, leftWidth, durWidth, old)
	})

	return b.addCommands(phaseNode, phaseLog, phaseI, entityVersion)
}

func (b *BuildLogs) addCommands(
	phaseNode *tree.Node,
	phaseLog *phaselogs.PhaseLog,
	p phase.Phase,
	entityVersion uint64,
) bool {
	hideable := b.isHideable(p)
	cmds := phaseLog.CommandLogs

	tasLoaded := phaseLog.TimeAndState.Load()
	if cmds == nil {
		return tasLoaded.EndError != nil
	}

	hasError := tasLoaded.EndError != nil

	cmdLen := cmds.Length()
	lastIdx := cmdLen - 1

	cmds.ForEach(func(idx int, cmd *command.CommandLog) bool {
		if cmd == nil {
			return true
		}

		if !b.conf.Flags.Tui.ShowAllBuildLogs && hideable && idx != lastIdx {
			return true
		}

		b.addCommand(phaseNode, cmd, idx, entityVersion)

		if cmd.TimeAndState != nil {
			t := cmd.TimeAndState.Load()
			if t != nil && t.EndError != nil {
				hasError = true
			}
		}

		return true
	})

	return hasError
}

func (b *BuildLogs) addCommand(parent *tree.Node, cmd *command.CommandLog, idx int, entityVersion uint64) {
	if cmd == nil || b.viewports == nil {
		return
	}

	cmdXpath := cmd.Xpath
	labelXpath := cmd.LabelXpath
	labelContent, labelVersion := b.commandLabelContent(cmd)

	displayVersion := entityVersion + cmd.TimeAndState.StateVersion()

	cmdNode := parent.Child(cmdXpath, displayVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
		tasCached := cmd.TimeAndState.Load()

		b.iconBuf.Reset()
		b.iconBuf.WriteString(strconv.Itoa(idx + 1))
		icon := b.spinnerOrIcon(cmdXpath, b.iconBuf.Bytes(), tasCached)

		b.cmdIconBuf.Reset()
		b.conf.ColorScheme.Command.Color.RenderLineInto(b.cmdIconBuf, icon)

		durStyled, durWidth := b.durationBytes(cmd.TimeAndState)

		iconWidth := style.CellWidth(b.cmdIconBuf.Line(0))

		labelCopy := make([]byte, len(labelContent))
		copy(labelCopy, labelContent)

		labelBuf := buffer.NewLinesBuf()
		labelBuf.WriteLine(labelCopy)

		labelWidth := depthWidth + iconWidth + durWidth

		labelResult := b.viewports.RenderLabelViewport(labelXpath, labelBuf, labelVersion, labelWidth)

		if cmd.Output.Len() > 0 {
			for range labelResult.Len() - 1 {
				b.conf.ColorScheme.Tree.Enumerator.RenderLineInto(b.cmdIconBuf, []byte("│"))
			}
		}

		b.cmdLabelBuf.Reset()

		for i := range labelResult.Len() {
			b.conf.ColorScheme.Command.Color.RenderLineInto(b.cmdLabelBuf, labelResult.Line(i))
		}

		b.cmdDurBuf.Reset()
		b.conf.ColorScheme.Command.Color.RenderLineInto(b.cmdDurBuf, durStyled)

		joinBuf := reuseBuf(old)
		style.JoinHorizontalBufs(joinBuf, style.Top, b.cmdIconBuf, b.cmdLabelBuf, b.cmdDurBuf)

		return joinBuf
	})

	b.addCommandChildren(cmdNode, cmd, cmd.TimeAndState.Load(), entityVersion)
}

func (b *BuildLogs) commandLabelContent(cmd *command.CommandLog) ([]byte, uint64) {
	if b.conf.Flags.Tui.ShowCommandsInLabels && cmd.Command != nil && cmd.Command.Len() > 2 {
		return cmd.Command.Bytes(), 1
	}

	return []byte(cmd.Description), 0
}

func (b *BuildLogs) addCommandChildren(
	cmdNode *tree.Node, cmd *command.CommandLog,
	tasCached *atomictimeandstate.TimeAndState,
	entityVersion uint64,
) {
	output := cmd.Output
	outputXpath := cmd.OutputXpath

	if output.Len() > 0 {
		ver := output.Version()
		cmdNode.Child(outputXpath, ver, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
			snap := output.Snapshot()
			outResult := b.viewports.RenderViewportVersioned(outputXpath, snap, ver, depthWidth)

			outBuf := reuseBuf(old)
			outBuf.AppendFrom(outResult)

			return outBuf
		})
	}

	errXpath := cmd.ErrorXpath

	err := tasCached.EndError
	if err != nil {
		cmdNode.Child(errXpath, entityVersion, func(depthWidth int, old *buffer.LinesBuf) *buffer.LinesBuf {
			errMsg := buffer.NewLineBufPooled()
			errMsg.Write(b.conf.ColorScheme.Chars.ErrorIcon)
			errMsg.WriteString(" Command failed: ")
			errMsg.WriteString(err.Error())

			errMsgCopy := make([]byte, errMsg.Len())
			copy(errMsgCopy, errMsg.Bytes())

			errBuf := buffer.NewLinesBuf()
			errBuf.WriteLine(errMsgCopy)
			errResult := b.viewports.RenderLabelViewport(errXpath, errBuf, 0, depthWidth)

			errMsg.Release()

			b.errBuf.Reset()
			b.conf.ColorScheme.Error.Color.RenderIntoBuf(b.errBuf, errResult)

			errNodeBuf := reuseBuf(old)
			errNodeBuf.AppendFrom(b.errBuf)

			return errNodeBuf
		})
	}
}

func (b *BuildLogs) entityNodeContent(
	indent int,
	entity colorscheme.ColorSchemeLogEntity,
	name string,
	logNode *logs.Logs,
	old *buffer.LinesBuf,
) *buffer.LinesBuf {
	var duration time.Duration
	if logNode != nil {
		duration = logNode.TAS.DurationCache
	}

	return b.entityNodeContentWithDuration(indent, entity, name, duration, old)
}

func (b *BuildLogs) entityNodeContentWithDuration(
	indent int,
	entity colorscheme.ColorSchemeLogEntity,
	name string,
	duration time.Duration,
	old *buffer.LinesBuf,
) *buffer.LinesBuf {
	dur := duration.Seconds()

	b.iconBuf.Reset()
	b.iconBuf.Write(entity.Icon)
	b.iconBuf.WriteByte(' ')
	b.iconBuf.WriteString(name)
	leftRaw := b.iconBuf.Bytes()

	rightRaw := b.formatDuration(dur)

	leftWidth := style.CellWidth(entity.Icon) + 1 + len(name)
	rightWidth := len(rightRaw)

	return b.layoutLineStyled(indent, indent/TreeStep, entity.Color, leftRaw, rightRaw, leftWidth, rightWidth, old)
}

func (b *BuildLogs) entityVersion(logNode *logs.Logs) uint64 {
	version := b.widthOffset

	if logNode != nil {
		version += logNode.Version()

		if !logNode.TAS.IsFinished() {
			version += b.spinners.Generation()
		}
	}

	return version
}

// layoutLineStyled renders styled left + pad + styled right into a node buffer.
func (b *BuildLogs) layoutLineStyled(
	indent, timerLevel int,
	sty style.Style,
	leftRaw, rightRaw []byte,
	leftWidth, rightWidth int,
	old *buffer.LinesBuf,
) *buffer.LinesBuf {
	available := b.contentWidth - indent - (timerIndent - timerLevel)
	pad := max(available-rightWidth-leftWidth, leftWidth)

	var padBytes []byte
	if pad <= maxSpaces {
		padBytes = spacesBytes[:pad]
	} else {
		padBytes = []byte(strings.Repeat(" ", pad))
	}

	buf := reuseBuf(old)
	buf.EmptyLine()
	sty.RenderAppend(buf, leftRaw)
	buf.AppendToLine(padBytes)
	sty.RenderAppend(buf, rightRaw)

	return buf
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

func (b *BuildLogs) durationBytes(tas *atomictimeandstate.AtomicTimeAndState) ([]byte, int) {
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
