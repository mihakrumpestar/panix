package tui

import (
	"fmt"
	"slices"
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

const treeStep = 3

var hideablePhases = []phases.Phase{phases.Inspect, phases.Secrets}

// ViewBuildLogs generates a tree view of all build logs
// The tree structure adapts based on what's selected:
// - If a machine is selected in stats table: show that machine's logs
// - If a phase is selected: show that phase across all machines
// - Otherwise: show default view (Inspect + remaining phases per scope)
func (m *model) ViewBuildLogs() string {
	var builder strings.Builder

	builder.WriteString(m.conf.ColorScheme.HeaderTitle.Render("=== Build Logs ===\n"))

	resetable := m.resetable.Load()
	resetable.workflow.State().TargetsLogs.CalculateDurationAndError()

	selectedXpath := resetable.statsTable.GetSelectedXpath()
	selectedPhase := resetable.phaseStatus.GetSelectedPhase()
	colors := m.conf.ColorScheme

	// Build tree for each flake
	for _, flakePair := range m.conf.Root.Flakes.Omap.Pairs() {
		flake := flakePair.Value
		flakeNode := m.createNode(0, colors.Flake, &flake.Attributes, true)

		// Add configurations under flake
		for _, cfgPair := range flake.Configurations.Omap.Pairs() {
			cfg := cfgPair.Value
			cfgNode := m.createNode(treeStep, colors.Configuration, &cfg.Attributes, false)
			machines := cfg.Machines.Omap.Pairs()

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
	indent := treeStep * 2
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

// addMachineTree adds a machine and all its phases to the config node
// Used when a specific machine is selected in the stats table
func (m *model) addMachineTree(cfgNode *tree.Tree, cfg *config.Configuration, machine *config.Machine) {
	indent := treeStep * 2
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

// addDefaultTree adds all phases in order from PhaseRegistry to the tree
// Respects scope: ScopeConfig phases at config level, ScopeMachine per machine
func (m *model) addDefaultTree(cfgNode *tree.Tree, cfg *config.Configuration, machines []omap.Pair[string, *config.Machine]) {
	indent := treeStep * 2

	for _, phaseMeta := range phases.PhaseRegistry {
		if phaseMeta.Scope == phases.ScopeConfig {
			// Config-scoped phase (like Build): use config attributes
			m.addPhases(cfgNode, &cfg.Attributes, indent, false, phaseMeta.Phase)
		} else {
			// Machine-scoped phase: add for each machine
			for _, pair := range machines {
				m.addMachinePhases(cfgNode, pair.Value, indent, phaseMeta.Phase)
			}
		}
	}
}

// addMachinePhases adds a machine node with specific phases to the parent tree
// Used when showing a specific phase or default view across all machines
func (m *model) addMachinePhases(parent *tree.Tree, machine *config.Machine, indent int, allowed ...phases.Phase) {
	node := m.createNode(indent, m.conf.ColorScheme.Machine, &machine.Attributes, false)
	m.addPhases(node, &machine.Attributes, indent+treeStep, false, allowed...)

	if node.Children().Length() > 0 {
		parent.Child(node)
	}
}

// createNode creates a tree node for an entity (flake/config/machine)
// Each node shows: icon + name + message (left aligned) and duration (right aligned)
func (m *model) createNode(indent int, style config.ColorSchemeLogEntity, attr *attributes.Attributes, isRoot bool) *tree.Tree {
	duration := m.resetable.Load().workflow.State().TargetsLogs.MustGet(attr.Xpath).GetCachedDurationAndError().Duration
	line := m.layoutLine(indent,
		style.Color.Render(fmt.Sprintf("%s %s %s", string(style.Icon), attr.Name, attr.Message)),
		style.Color.Render(fmt.Sprintf("(%.2fs)", duration.Seconds())))

	treeInst := tree.New().Root(line)
	if isRoot {
		treeInst = treeInst.Enumerator(tree.RoundedEnumerator).
			EnumeratorStyle(m.conf.ColorScheme.TreeEnumerator).
			IndenterStyle(m.conf.ColorScheme.TreeEnumerator)
	}

	return treeInst
}

// addPhases adds phase nodes to a parent tree for specific phases
// Returns true if an error was encountered (for stopAtError logic)
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

// addPhase adds a single phase node with its commands to the parent tree
// Hides successful hideable phases unless ShowAllBuildLogs is true
func (m *model) addPhase(parent *tree.Tree, attr *attributes.Attributes, phase phases.Phase, phaseLog *phase.PhaseLog, indent int) bool {
	if phaseLog == nil {
		return false
	}

	tas := phaseLog.TimeAndState()
	hideable := slices.Contains(hideablePhases, phase)
	shouldHide := (!m.conf.Flags.Tui.ShowAllBuildLogs && hideable) || m.conf.Flags.Tui.ShowActiveOnly

	if shouldHide && tas.IsFinished() && tas.GetEndError() == nil {
		return false
	}

	colors := m.conf.ColorScheme
	phaseXpath := attr.Xpath.NewXpathWithAppend(string(phase))
	icon := m.spinnerOrIcon(phaseXpath, string(colors.Phase.Icon), tas)
	duration := m.durationText(colors.Phase, tas)
	line := m.layoutLine(indent, colors.Phase.Color.Render(icon+" "+strings.ToUpper(string(phase))), duration)
	phaseNode := tree.New().Root(colors.Phase.Color.Render(line))

	cmds := phaseLog.CommandLogs()
	hasError := tas.GetEndError() != nil

	for i, cmd := range cmds {
		if m.conf.Flags.Tui.ShowAllBuildLogs || !hideable || i == len(cmds)-1 {
			m.addCommand(phaseNode, cmd, i, phaseXpath, indent)

			if cmd.TimeAndState.GetEndError() != nil {
				hasError = true
			}
		}
	}

	parent.Child(phaseNode)

	return hasError
}

// addCommand adds a command node with its output and errors to the phase tree
// Each command shows: index + description/command + duration
// If output exists, it wraps in a scrollable viewport
func (m *model) addCommand(parent *tree.Tree, cmd *command.CommandLog, idx int, phaseXpath attributes.Xpath, indent int) {
	colors := m.conf.ColorScheme
	cmdIndent := indent + treeStep
	resetable := m.resetable.Load()

	// Use command or description based on flag
	label := cmd.Description
	if m.conf.Flags.Tui.ShowCommandsInLabels {
		label = cmd.Command
	}

	cmdXpath := phaseXpath.NewXpathWithAppend(label)
	icon := m.spinnerOrIcon(cmdXpath, fmt.Sprintf("%d ", idx+1), cmd.TimeAndState)
	duration := m.durationText(colors.Command, cmd.TimeAndState)

	// Create label viewport for wrapping long labels
	labelViewport := resetable.viewports.GetOrCreateLabelViewport(cmdXpath.NewXpathWithAppend("label"), label, cmdIndent+lipgloss.Width(icon)+lipgloss.Width(duration))

	// If label wraps multiple lines, extend icon with tree lines for alignment
	output := strings.TrimSpace(cmd.String())
	if len(output) > 0 && lipgloss.Height(labelViewport) > 1 {
		icon = lipgloss.JoinVertical(lipgloss.Left, append([]string{icon},
			slices.Repeat([]string{colors.TreeEnumerator.Render("│")}, lipgloss.Height(labelViewport)-1)...)...)
	}

	cmdNode := tree.New().Root(colors.Command.Color.Render(lipgloss.JoinHorizontal(lipgloss.Top, icon, colors.Command.Color.Render(labelViewport), duration)))

	// Add command output in a viewport if it exists
	if len(output) > 0 {
		cmdNode.Child(resetable.viewports.GetOrCreateViewport(cmdXpath.NewXpathWithAppend("output"), output, cmdIndent+treeStep*2-1))
	} else {
		resetable.viewports.RemoveIfExistsViewport(cmdXpath.NewXpathWithAppend("output"))
	}

	// Add error message if command failed
	if err := cmd.TimeAndState.GetEndError(); err != nil {
		cmdNode.Child(colors.Error.Color.Render("✗ Command failed: " + err.Error()))
	}

	parent.Child(cmdNode)
}

// layoutLine creates a line with left-aligned content and right-aligned duration
// Calculates available width minus indent and timer indent
func (m *model) layoutLine(indent int, left, right string) string {
	timerIndent := max(4-indent/treeStep, 0)
	available := m.resetable.Load().viewports.ContentWidth() - indent - timerIndent
	rightWidth := lipgloss.Width(right)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(max(available-rightWidth, lipgloss.Width(left))).Render(left),
		right)
}

// spinnerOrIcon returns an icon or spinner based on execution state
// Shows spinner if running, icon if finished, empty string if not started
func (m *model) spinnerOrIcon(xpath attributes.Xpath, icon string, tas *timeandstate.TimeAndState) string {
	if !tas.HasStarted() {
		return ""
	}

	if tas.IsFinished() {
		return icon
	}

	return m.resetable.Load().spinners.GetOrCreateSpinner(xpath).View()
}

// durationText formats duration text with proper styling
// Returns empty string if execution hasn't started
func (m *model) durationText(style config.ColorSchemeLogEntity, tas *timeandstate.TimeAndState) string {
	if d, err := tas.DurationOrElapsedTime(); err == nil {
		return style.Color.PaddingLeft(1).Render(fmt.Sprintf("(%.2fs)", d.Seconds()))
	}

	return ""
}
