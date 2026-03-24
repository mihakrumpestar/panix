package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/kirill-scherba/omap"
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

// ViewBuildLogs generates a tree view of all build logs.
// The tree structure adapts based on what's selected:
// - If a machine is selected in stats table: show that machine's logs
// - If a phase is selected: show that phase across all machines
// - Otherwise: show default view (Inspect + remaining phases per scope).
func (m *model) ViewBuildLogs() string {
	var builder strings.Builder

	builder.WriteString(m.conf.ColorScheme.Header.Title.Render("=== Build Logs ===\n"))

	resetable := m.resetable.Load()
	resetable.workflow.State().TargetsLogs.CalculateDurationAndError()

	selectedXpath := resetable.statsTable.GetSelectedXpath()
	selectedPhase := resetable.phaseStatus.GetSelectedPhase()
	colors := m.conf.ColorScheme

	// Build tree for each flake
	for _, flakePair := range m.conf.Root.Flakes.Pairs() {
		flake := flakePair.Value
		flakeNode := m.createNode(0, colors.Flake, &flake.Attributes, true)

		// Add configurations under flake
		for _, cfgPair := range flake.Configurations.Pairs() {
			cfg := cfgPair.Value
			cfgNode := m.createNode(treeStep, colors.Configuration, &cfg.Attributes, false)
			machines := cfg.Machines.Pairs()

			// Determine what to show based on selection
			switch {
			case selectedXpath.Depth() > 0:
				// Show only selected machine's logs
				for _, pair := range machines {
					if pair.Value.Xpath == selectedXpath {
						m.addMachineTree(cfgNode, cfg, pair.Value)

						break
					}
				}
			case selectedPhase != "":
				// Show selected phase across relevant machines/config
				m.addPhaseToTree(cfgNode, cfg, machines, selectedPhase)
			default:
				// Default view: phases in order from PhaseRegistry, respecting scope
				m.addDefaultTree(cfgNode, cfg, machines)
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

func (m *model) addPhaseToTree(cfgNode *tree.Tree, cfg *config.Configuration, machines []omap.Pair[string, *config.Machine], phase phases.Phase) {
	indent := treeStep * indentStep
	// ScopeConfig phases (like Build) run once per configuration
	// ScopeMachine phases run per machine
	if phases.GetPhaseScope(phase) == phases.ScopeConfig {
		m.addPhases(cfgNode, &cfg.Attributes, indent, false, phase)
	} else {
		for _, pair := range machines {
			m.addMachinePhases(cfgNode, pair.Value, indent, phase)
		}
	}
}

// addMachineTree adds a machine and all its phases to the config node.
// Used when a specific machine is selected in the stats table.
func (m *model) addMachineTree(cfgNode *tree.Tree, cfg *config.Configuration, machine *config.Machine) {
	indent := treeStep * indentStep
	node := m.createNode(indent, m.conf.ColorScheme.Machine, &machine.Attributes, false)

	// Add Inspect phase (stops at first error)
	errored := m.addPhases(node, &machine.Attributes, indent+treeStep, true, phases.Inspect)
	if !errored {
		// Add remaining phases after Inspect, using appropriate attributes based on scope
		for _, phaseMeta := range phases.PhaseRegistry[1:] {
			attr := &machine.Attributes
			if phaseMeta.Scope == phases.ScopeConfig {
				attr = &cfg.Attributes
			}

			m.addPhases(node, attr, indent+treeStep, true, phaseMeta.Phase)
		}
	}

	if node.Children().Length() > 0 {
		cfgNode.Child(node)
	}
}

// addDefaultTree adds all phases in order from PhaseRegistry to the tree.
// Respects scope: ScopeConfig phases at config level, ScopeMachine per machine.
// Consecutive machine-scoped phases are grouped under the same machine node.
// When a config-scoped phase appears, it breaks the grouping, and subsequent
// machine-scoped phases create new machine nodes.
func (m *model) addDefaultTree(cfgNode *tree.Tree, cfg *config.Configuration, machines []omap.Pair[string, *config.Machine]) {
	indent := treeStep * indentStep

	var pendingMachinePhases []phases.Phase

	flushMachinePhases := func() {
		if len(pendingMachinePhases) == 0 {
			return
		}

		for _, pair := range machines {
			m.addMachinePhases(cfgNode, pair.Value, indent, pendingMachinePhases...)
		}

		pendingMachinePhases = nil
	}

	for _, phaseMeta := range phases.PhaseRegistry {
		if phaseMeta.Scope == phases.ScopeConfig {
			flushMachinePhases()
			m.addPhases(cfgNode, &cfg.Attributes, indent, false, phaseMeta.Phase)
		} else {
			pendingMachinePhases = append(pendingMachinePhases, phaseMeta.Phase)
		}
	}

	flushMachinePhases()
}

// addMachinePhases adds a machine node with specific phases to the parent tree.
// Used when showing a specific phase or default view across all machines.
func (m *model) addMachinePhases(parent *tree.Tree, machine *config.Machine, indent int, allowed ...phases.Phase) {
	node := m.createNode(indent, m.conf.ColorScheme.Machine, &machine.Attributes, false)
	m.addPhases(node, &machine.Attributes, indent+treeStep, false, allowed...)

	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

// createNode creates a tree node for an entity (flake/config/machine).
// Each node shows: icon + name + message (left aligned) and duration (right aligned).
func (m *model) createNode(indent int, style config.ColorSchemeLogEntity, attr *attributes.Attributes, isRoot bool) *tree.Tree {
	duration := m.resetable.Load().workflow.State().TargetsLogs.MustGet(attr.Xpath).GetCachedDurationAndError().Duration

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

// addPhases adds phase nodes to a parent tree for specific phases.
// Returns true if an error was encountered (for stopAtError logic).
func (m *model) addPhases(parent *tree.Tree, attr *attributes.Attributes, indent int, stopAtError bool, allowed ...phases.Phase) bool {
	logs := m.resetable.Load().workflow.State().TargetsLogs.MustGetLogs(attr.Xpath)
	for _, entry := range logs.All() {
		if !slices.Contains(allowed, entry.Key) {
			continue
		}

		if m.addPhase(parent, attr, entry.Key, entry.Value, indent) && stopAtError {
			return true
		}
	}

	return false
}

func (m *model) addPhase(parent *tree.Tree, attr *attributes.Attributes, phase phases.Phase, phaseLog *phase.PhaseLog, indent int) bool {
	if phaseLog == nil {
		return false
	}

	if m.shouldHidePhase(phase, phaseLog) {
		return false
	}

	phaseNode := m.createPhaseNode(attr, phase, phaseLog, indent)
	hasError := m.addCommandsToPhase(phaseNode, phaseLog, phase, attr, indent)
	parent.Child(phaseNode)

	return hasError
}

func (m *model) shouldHidePhase(phase phases.Phase, phaseLog *phase.PhaseLog) bool {
	hideable := slices.Contains(hideablePhases, phase)
	shouldHide := (!m.conf.Flags.Tui.ShowAllBuildLogs && hideable) || m.conf.Flags.Tui.ShowActiveOnly

	return shouldHide && phaseLog.TimeAndState().IsFinished() && phaseLog.TimeAndState().GetEndError() == nil
}

func (m *model) createPhaseNode(attr *attributes.Attributes, phase phases.Phase, phaseLog *phase.PhaseLog, indent int) *tree.Tree {
	colors := m.conf.ColorScheme
	phaseXpath := attr.Xpath.NewXpathWithAppend(string(phase))
	tas := phaseLog.TimeAndState()

	icon := m.spinnerOrIcon(phaseXpath, string(colors.Phase.Icon), tas)
	durationStyled, durationWidth := m.durationText(colors.Phase, tas)

	leftRaw := icon + strings.ToUpper(string(phase))
	left := colors.Phase.Color.Render(leftRaw)

	line := m.layoutLine(indent, left, durationStyled, len(leftRaw), durationWidth)

	return tree.New().Root(colors.Phase.Color.Render(line))
}

func (m *model) addCommandsToPhase(phaseNode *tree.Tree, phaseLog *phase.PhaseLog, phase phases.Phase, attr *attributes.Attributes, indent int) bool {
	hideable := slices.Contains(hideablePhases, phase)
	phaseXpath := attr.Xpath.NewXpathWithAppend(string(phase))
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

// addCommand adds a command node with its output and errors to the phase tree.
// Each command shows: index + description/command + duration.
// If output exists, it wraps in a scrollable viewport.
func (m *model) addCommand(parent *tree.Tree, cmd *command.CommandLog, idx int, phaseXpath attributes.Xpath, indent int) {
	colors := m.conf.ColorScheme
	cmdIndent := indent + treeStep
	resetable := m.resetable.Load()

	label := cmd.Description

	// Use command or description based on flag
	command := cmd.Command
	if m.conf.Flags.Tui.ShowCommandsInLabels && len(command) > 2 { // Don't show "command" if it is implemented in Golang instead of shell
		label = command
	}

	cmdXpath := phaseXpath.NewXpathWithAppend(label)
	icon := m.spinnerOrIcon(cmdXpath, strconv.Itoa(idx+1), cmd.TimeAndState)
	durationStyled, durationWidth := m.durationText(colors.Command, cmd.TimeAndState)

	// Create label viewport for wrapping long labels
	labelWidth := cmdIndent + lipgloss.Width(icon) + durationWidth
	labelViewport := resetable.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("label"), label, labelWidth)
	labelViewportHeight := lipgloss.Height(labelViewport)

	// If label wraps multiple lines, extend icon with tree lines for alignment
	if labelViewportHeight > 1 {
		treeLine := "\n" + colors.Tree.Enumerator.Render("│")
		icon += strings.Repeat(treeLine, labelViewportHeight-1)
	}

	cmdNode := tree.New().Root(colors.Command.Color.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, icon, colors.Command.Color.Render(labelViewport), durationStyled),
	))

	// Add command output in a viewport if it exists
	output := cmd.StringForBuildLogs()
	if len(output) > 0 {
		cmdNode.Child(resetable.viewports.GetOrCreateViewport(cmdXpath.NewXpathWithAppend("output"), output, cmdIndent+treeStep*2-1))
	} else {
		resetable.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("output"))
	}

	// Add error message if command failed
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

// layoutLine creates a line with left-aligned content and right-aligned duration.
// Timer indentation from right: flake=4, config=3, machine=2, phase=1, command=0.
// indent accounts for the tree prefix width added by the tree library.
func (m *model) layoutLine(indent int, left, right string, leftWidth, rightWidth int) string {
	leftWidth -= 2 // Due to miscalculation with emoji chars

	level := indent / treeStep
	timerIndentFromRight := timerIndent - level
	available := m.resetable.Load().viewports.ContentWidth() - indent - timerIndentFromRight
	centerSpace := strings.Repeat(" ", max(available-rightWidth-leftWidth, leftWidth))

	return left + centerSpace + right
}

// spinnerOrIcon returns an icon or spinner based on execution state.
// Shows spinner if running, icon if finished, empty string if not started.
func (m *model) spinnerOrIcon(xpath attributes.Xpath, icon string, tas *timeandstate.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		return icon + " " // Spinner seems to add this by itself
	}

	return m.resetable.Load().spinners.GetOrCreateSpinner(xpath).View()
}

// durationText formats duration text with proper styling.
// Returns the styled string and its width.
func (m *model) durationText(style config.ColorSchemeLogEntity, tas *timeandstate.TimeAndState) (string, int) {
	duration, err := tas.DurationOrElapsedTime()
	if err == nil {
		text := fmt.Sprintf(" (%.2fs)", duration.Seconds())
		styled := style.Color.Render(text)

		return styled, len(text)
	}

	return "", 0
}
