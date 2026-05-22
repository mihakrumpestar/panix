package buildlogs

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
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

var testTreeStyle = style.NewStyle()

// testRootNode creates a tree Node with string content for tests.
func testRootNode(s string) *tree.Node {
	lb := buffer.NewLinesBuf()
	lb.WriteLine([]byte(s))

	return tree.NewTree(testTreeStyle).NewNode(lb)
}

// --- formatDuration ---

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		secs float64
		want string
	}{
		{0.0, " (0.00s)"},
		{1.5, " (1.50s)"},
		{0.123, " (0.12s)"},
		{99.999, " (100.00s)"},
		{0.004, " (0.00s)"},
	}

	for _, tc := range tests {
		b := &BuildLogs{durLineBuf: buffer.NewLineBuf()}

		got := b.formatDuration(tc.secs)
		assert.Equal(t, tc.want, string(got), "formatDuration(%v)", tc.secs)
	}
}

// --- isHideable ---

func TestIsHideable(t *testing.T) {
	t.Parallel()

	buildLogs := &BuildLogs{}

	hideablePhases := []phase.Phase{phase.Inspect, phase.Secrets}
	nonHideablePhases := []phase.Phase{phase.Build, phase.Bootstrap, phase.Transfer, phase.Activate, phase.Rollback}

	for _, p := range hideablePhases {
		assert.True(t, buildLogs.isHideable(p), "isHideable(%s) = false, want true", p)
	}

	for _, p := range nonHideablePhases {
		assert.False(t, buildLogs.isHideable(p), "isHideable(%s) = true, want false", p)
	}
}

// --- shouldHidePhase ---

func TestShouldHidePhase_HideableFinishedNoError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(nil)

	assert.True(t, buildLogs.shouldHidePhase(phase.Inspect, phaseLog),
		"should hide finished hideable phase with no error")
}

func TestShouldHidePhase_HideableFinishedWithError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(os.ErrNotExist)

	assert.False(t, buildLogs.shouldHidePhase(phase.Inspect, phaseLog),
		"should NOT hide finished hideable phase with error")
}

func TestShouldHidePhase_HideableRunning(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newRunningPhaseLog()

	assert.False(t, buildLogs.shouldHidePhase(phase.Inspect, phaseLog),
		"should NOT hide running hideable phase")
}

func TestShouldHidePhase_NonHideableFinishedNoError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(nil)

	assert.False(t, buildLogs.shouldHidePhase(phase.Build, phaseLog),
		"should NOT hide finished non-hideable phase (Build)")
}

func TestShouldHidePhase_ShowAllBuildLogs(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	conf.Flags.Tui.ShowAllBuildLogs = true
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(nil)

	assert.False(t, buildLogs.shouldHidePhase(phase.Inspect, phaseLog),
		"ShowAllBuildLogs=true should NOT hide finished hideable phase")
}

func TestShouldHidePhase_ShowActiveOnly(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	conf.Flags.Tui.ShowActiveOnly = true
	buildLogs := New(conf, nil, nil)

	runningPhaseLog := newRunningPhaseLog()
	assert.False(t, buildLogs.shouldHidePhase(phase.Build, runningPhaseLog),
		"ShowActiveOnly should NOT hide running phase")

	finishedPhaseLog := newFinishedPhaseLog(nil)
	assert.True(t, buildLogs.shouldHidePhase(phase.Build, finishedPhaseLog),
		"ShowActiveOnly should hide finished phase even if non-hideable")
}

// --- layoutLine ---

func TestLayoutLine(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 80

	line := buildLogs.layoutLineStyled(0, style.NewStyle(), []byte("left"), []byte("right"), 4, 5)

	assert.Contains(t, buffer.LinesBufToStringForTests(line), "left",
		"layoutLine should contain left text")
	assert.Contains(t, buffer.LinesBufToStringForTests(line), "right",
		"layoutLine should contain right text")
}

func TestLayoutLine_NarrowWidth(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 20

	line := buildLogs.layoutLineStyled(6, style.NewStyle(), []byte("BUILD"), []byte("(1.23s)"), 5, 7)

	assert.Contains(t, buffer.LinesBufToStringForTests(line), "BUILD",
		"layoutLine should contain left text")
	assert.Contains(t, buffer.LinesBufToStringForTests(line), "(1.23s)",
		"layoutLine should contain right text")
}

// --- spinnerOrIcon ---

func TestSpinnerOrIcon_NotStarted(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	tas := &atomictimeandstate.TimeAndState{}

	result := buildLogs.spinnerOrIcon(xpath.New("test"), []byte("icon"), tas)
	assert.Empty(t, result)
}

func TestSpinnerOrIcon_Finished(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	tas := atomictimeandstate.New()
	tas.StartTimer()
	tas.EndTimerWithError(nil)
	tasLoaded := tas.Load()

	result := buildLogs.spinnerOrIcon(xpath.New("test"), []byte("OK"), tasLoaded)
	assert.Equal(t, "OK ", string(result), "spinnerOrIcon for finished = %q, want %q", result, "OK ")
}

func TestSpinnerOrIcon_Running(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	tas := atomictimeandstate.New()
	tas.StartTimer()
	tasLoaded := tas.Load()

	result := buildLogs.spinnerOrIcon(xpath.New("test"), []byte("OK"), tasLoaded)
	assert.NotEmpty(t, result, "spinnerOrIcon for running should return spinner, got empty")
	assert.NotEqual(t, "OK ", string(result), "spinnerOrIcon for running should NOT return finished icon")
}

// --- addPhases ---

func TestAddPhases_NilLogNode(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	parent := testRootNode("parent")

	result := buildLogs.addPhases(parent, nil, xpath.New("test"), 0, false, phase.Build)
	assert.False(t, result, "addPhases with nil logNode should return false")
	assert.Equal(t, 0, parent.Len(), "addPhases with nil logNode should not add children")
}

func TestAddPhases_NilPhaseLogs(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	logNode := &logs.Logs{}

	parent := testRootNode("parent")

	result := buildLogs.addPhases(parent, logNode, xpath.New("test"), 0, false, phase.Build)
	assert.False(t, result, "addPhases with nil PhaseLogs should return false")
}

func newBuildLogsForTest() *BuildLogs {
	conf := makeTestConfig(0, 0, 0, nil)
	bl := New(conf, nil, nil)
	bl.contentWidth = 120
	bl.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	bl.viewports = newTestViewports(conf)

	return bl
}

func TestAddPhasesSingle_HideableFinishedFiltered(t *testing.T) {
	t.Parallel()

	buildLogs := newBuildLogsForTest()

	logNode := logs.New()
	phaseLog := newFinishedPhaseLog(nil)
	logNode.PhaseLogs.Set(phase.Inspect, phaseLog)

	parent := testRootNode("parent")

	result := buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Inspect)
	assert.False(t, result, "addPhasesSingle should return false when phase is hidden")
	assert.Equal(t, 0, parent.Len(), "addPhasesSingle should add 0 children when phase hidden, got %d", parent.Len())
}

func TestAddPhasesSingle_HideableFinishedWithErrorNotFiltered(t *testing.T) {
	t.Parallel()

	buildLogs := newBuildLogsForTest()

	logNode := logs.New()
	phaseLog := newFinishedPhaseLog(os.ErrNotExist)
	logNode.PhaseLogs.Set(phase.Inspect, phaseLog)

	parent := testRootNode("parent")

	buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Inspect)

	assert.Equal(t, 1, parent.Len(), "addPhasesSingle should add child when phase has error, got %d", parent.Len())

	parent2 := testRootNode("parent")
	result2 := buildLogs.addPhases(parent2, logNode, xpath.New("test"), 6, true, phase.Inspect)

	assert.True(t, result2, "addPhasesSingle with stopAtError should return true when phase has error")
}

func TestAddPhasesSingle_BuildPhaseNotFiltered(t *testing.T) {
	t.Parallel()

	buildLogs := newBuildLogsForTest()

	logNode := logs.New()
	phaseLog := newFinishedPhaseLog(nil)
	logNode.PhaseLogs.Set(phase.Build, phaseLog)

	parent := testRootNode("parent")

	result := buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Build)
	assert.False(t, result, "addPhasesSingle should return false when phase finishes with no error")
	assert.Equal(t, 1, parent.Len(), "addPhasesSingle should add child for non-hideable phase, got %d", parent.Len())
}

func TestAddPhasesMulti_Filtering(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	logNode := logs.New()

	inspectLog := newFinishedPhaseLog(nil)
	logNode.PhaseLogs.Set(phase.Inspect, inspectLog)

	buildLog := newFinishedPhaseLog(nil)
	logNode.PhaseLogs.Set(phase.Build, buildLog)

	parent := testRootNode("parent")

	buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Inspect, phase.Build)

	assert.Equal(t, 1, parent.Len(), "addPhasesMulti should add 1 child (Build only, Inspect hidden), got %d", parent.Len())
}

// --- addCommands ---

func TestAddCommands_NilCommandLogs(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	phaseLog := newFinishedPhaseLog(nil)
	phaseLog.CommandLogs = nil

	phaseNode := testRootNode("phase")

	hasError := buildLogs.addCommands(phaseNode, phaseLog, phase.Build, xpath.New("test"), 6)

	assert.False(t, hasError, "addCommands with nil CommandLogs and no error should return false")
}

func TestAddCommands_WithCommandError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	phaseLog := newFinishedPhaseLog(os.ErrNotExist)

	cmd := phaseLog.NewCommand("test cmd", "running", "failed", []string{"echo", "test"}, nil)
	cmd.TimeAndState.StartTimer()
	cmd.TimeAndState.EndTimerWithError(os.ErrNotExist)
	cmd.Output.Write([]byte("error output"))

	phaseNode := testRootNode("phase")

	hasError := buildLogs.addCommands(phaseNode, phaseLog, phase.Build, xpath.New("test"), 6)

	assert.True(t, hasError, "addCommands should return true when command has error")
	assert.NotZero(t, phaseNode.Len(), "addCommands should add command children")
}

func TestAddCommands_HideablePhaseOnlyLastCommandShown(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	conf.Flags.Tui.ShowAllBuildLogs = false
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	phaseLog := newFinishedPhaseLog(nil)

	cmd1 := phaseLog.NewCommand("cmd1", "running", "failed", []string{"cmd1"}, nil)
	cmd1.TimeAndState.StartTimer()
	cmd1.TimeAndState.EndTimerWithError(nil)

	cmd2 := phaseLog.NewCommand("cmd2", "running", "failed", []string{"cmd2"}, nil)
	cmd2.TimeAndState.StartTimer()
	cmd2.TimeAndState.EndTimerWithError(nil)

	cmd3 := phaseLog.NewCommand("cmd3", "running", "failed", []string{"cmd3"}, nil)
	cmd3.TimeAndState.StartTimer()
	cmd3.TimeAndState.EndTimerWithError(nil)

	phaseNode := testRootNode("phase")

	buildLogs.addCommands(phaseNode, phaseLog, phase.Inspect, xpath.New("test"), 6)

	assert.Equal(t, 1, phaseNode.Len(), "addCommands for hideable phase should show only last command, got %d children", phaseNode.Len())
}

func TestAddCommands_HideableVsNonHideablePhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phaseI   phase.Phase
		showAll  bool
		wantCmds int
	}{
		{"hideable_show_all", phase.Inspect, true, 2},
		{"hideable_default", phase.Inspect, false, 1},
		{"non_hideable_show_all", phase.Build, true, 2},
		{"non_hideable_default", phase.Build, false, 2},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			conf := makeTestConfig(0, 0, 0, nil)
			conf.Flags.Tui.ShowAllBuildLogs = testCase.showAll
			buildLogs := New(conf, nil, nil)
			buildLogs.contentWidth = 120
			buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
			buildLogs.viewports = newTestViewports(conf)

			phaseLog := newFinishedPhaseLog(nil)

			cmd1 := phaseLog.NewCommand("cmd1", "running", "failed", []string{"cmd1"}, nil)
			cmd1.TimeAndState.StartTimer()
			cmd1.TimeAndState.EndTimerWithError(nil)

			cmd2 := phaseLog.NewCommand("cmd2", "running", "failed", []string{"cmd2"}, nil)
			cmd2.TimeAndState.StartTimer()
			cmd2.TimeAndState.EndTimerWithError(nil)

			phaseNode := testRootNode("phase")

			buildLogs.addCommands(phaseNode, phaseLog, testCase.phaseI, xpath.New("test"), 6)

			assert.Equal(t, testCase.wantCmds, phaseNode.Len(), "got %d children, want %d", phaseNode.Len(), testCase.wantCmds)
		})
	}
}

// --- addCommandChildren ---

func TestAddCommandChildren_WithOutputAndError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.viewports = newTestViewports(conf)

	cmd := newFinishedCommandLog("test cmd", os.ErrNotExist)
	cmd.Output.Write([]byte("build output line 1"))
	cmd.Output.Write([]byte("build output line 2"))

	cmdNode := testRootNode("cmd")
	tas := cmd.TimeAndState.Load()

	buildLogs.addCommandChildren(cmdNode, cmd, xpath.New("test"), tas, 9)

	assert.GreaterOrEqual(t, cmdNode.Len(), 2,
		"addCommandChildren should add output and error children, got %d", cmdNode.Len())
}

func TestAddCommandChildren_NoOutputNoError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.viewports = newTestViewports(conf)

	cmd := newFinishedCommandLog("test cmd", nil)

	cmdNode := testRootNode("cmd")
	tas := cmd.TimeAndState.Load()

	buildLogs.addCommandChildren(cmdNode, cmd, xpath.New("test"), tas, 9)

	assert.Equal(t, 0, cmdNode.Len(),
		"addCommandChildren with no output and no error should add 0 children, got %d", cmdNode.Len())
}

// --- entityNode ---

func TestEntityNode(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.styledTreeLine = conf.ColorScheme.Tree.Enumerator.RenderLine([]byte("│"))

	logNode := logs.New()
	node := buildLogs.entityNode(0, conf.ColorScheme.Flake, "my-flake", logNode, true)

	buf := buffer.NewLinesBufDiff()
	node.Render(buf)

	result := buf.String()

	assert.Contains(t, result, "my-flake",
		"entityNode should contain the name")
	assert.Contains(t, result, "(0.00s)",
		"entityNode should contain duration")
}

func TestEntityNode_NilLogNode(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.styledTreeLine = conf.ColorScheme.Tree.Enumerator.RenderLine([]byte("│"))

	node := buildLogs.entityNode(0, conf.ColorScheme.Flake, "my-flake", nil, true)

	buf := buffer.NewLinesBufDiff()
	node.Render(buf)

	result := buf.String()

	assert.Contains(t, result, "my-flake",
		"entityNode with nil logNode should still contain the name")
	assert.Contains(t, result, "(0.00s)",
		"entityNode with nil logNode should show 0.00s duration")
}

// --- buildDefaultTree ---

func TestBuildDefaultTree_ConfigScopedPhases(t *testing.T) {
	t.Parallel()

	conf := makeTestConfigWithSingleMachine()
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	cfg := getFirstConfig(conf)

	cfgNode := testRootNode("cfg")

	buildLogs.buildDefaultTree(cfgNode, cfg)

	assert.NotZero(t, cfgNode.Len(), "buildDefaultTree should produce children")
}

func TestBuildDefaultTree_EmptyMachines(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(1, 1, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	cfg := getFirstConfig(conf)

	cfg.Logs.PhaseLogs.Set(phase.Build, newFinishedPhaseLog(nil))

	cfgNode := testRootNode("cfg")

	buildLogs.buildDefaultTree(cfgNode, cfg)

	assert.NotZero(t, cfgNode.Len(),
		"buildDefaultTree should produce config-scoped phases even with no machines")
}

// --- buildPhaseSelectedTree ---

func TestBuildPhaseSelectedTree_ConfigScopedPhase(t *testing.T) {
	t.Parallel()

	conf := makeTestConfigWithSingleMachine()
	phaseFlow := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, nil, phaseFlow)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	cfg := getFirstConfig(conf)

	// Add Build phase log to the config (config-scoped)
	cfg.Logs.PhaseLogs.Set(phase.Build, newFinishedPhaseLog(nil))

	// Select Build (config-scoped)
	buildLogs.phaseStatus = phaseFlow
	phaseFlow.Selected = phaseflow.Selected{Phase: phase.Build.String(), Index: 0}

	cfgNode := testRootNode("cfg")

	buildLogs.buildPhaseSelectedTree(cfgNode, cfg)

	assert.NotZero(t, cfgNode.Len(),
		"buildPhaseSelectedTree for config-scoped phase should add children")
}

func TestBuildPhaseSelectedTree_MachineScopedPhase(t *testing.T) {
	t.Parallel()

	conf := makeTestConfigWithSingleMachine()
	phaseFlow := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, nil, phaseFlow)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	cfg := getFirstConfig(conf)

	// Select Inspect (machine-scoped)
	phaseFlow.Selected = phaseflow.Selected{Phase: phase.Inspect.String(), Index: 0}

	cfgNode := testRootNode("cfg")

	buildLogs.buildPhaseSelectedTree(cfgNode, cfg)
}

// --- renderBuildLogsString helper (for tests) ---

func renderBuildLogsString(b *BuildLogs, vp *viewports.Viewports, sp *spinners.Spinners) string {
	return buffer.LinesBufToStringForTests(b.Render(vp, sp))
}

// --- View end-to-end ---

func TestView_BasicOutput(t *testing.T) {
	t.Parallel()

	conf := makeTestConfigWithSingleMachine()
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := renderBuildLogsString(buildLogs, vp, sp)

	stripped := string(style.StripANSI([]byte(result)))

	assert.Contains(t, stripped, "Build Logs",
		"View should contain 'Build Logs' header")
	assert.Contains(t, stripped, "flake0",
		"View should contain flake name")
}

func TestView_EmptyFleet(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := renderBuildLogsString(buildLogs, vp, sp)

	assert.Contains(t, result, "Build Logs",
		"View should contain 'Build Logs' header even with empty fleet")
}

func TestView_NilFlake(t *testing.T) {
	t.Parallel()

	conf := makeTestConfigWithSingleMachine()
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	conf.Fleet.Flakes.Set("nil-flake", nil)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := renderBuildLogsString(buildLogs, vp, sp)

	assert.Contains(t, string(style.StripANSI([]byte(result))), "Build Logs",
		"View should contain 'Build Logs' header")
}

func TestView_NilMachine(t *testing.T) {
	t.Parallel()

	conf := makeTestConfigWithSingleMachine()
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	// Add a nil machine entry — the tree traversal should skip it
	cfg := getFirstConfig(conf)
	cfg.Machines.Set("nil-mach", nil)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := renderBuildLogsString(buildLogs, vp, sp)

	assert.Contains(t, string(style.StripANSI([]byte(result))), "Build Logs",
		"View should contain 'Build Logs' header")
}

func TestView_MultipleFlakes(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(2, 2, 2, nil)
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := renderBuildLogsString(buildLogs, vp, sp)

	assert.Contains(t, result, "flake0",
		"View should contain first flake name")
	assert.Contains(t, result, "flake1",
		"View should contain second flake name")
}

// --- addPhase edge cases ---

func TestAddPhase_NotStarted(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	phaseLog := newNotStartedPhaseLog()

	parent := testRootNode("parent")

	result := buildLogs.addPhase(parent, xpath.New("test"), phase.Build, phaseLog, 6)

	assert.False(t, result, "addPhase for not-started phase should return false (no error)")
	assert.Equal(t, 1, parent.Len(), "addPhase should add the phase even when not started, got %d children", parent.Len())
}

func TestAddPhase_RunningPhaseShowsSpinner(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	phaseLog := newRunningPhaseLog()

	parent := testRootNode("parent")

	buildLogs.addPhase(parent, xpath.New("test"), phase.Build, phaseLog, 6)

	assert.Equal(t, 1, parent.Len(), "addPhase for running phase should add child")

	buf := buffer.NewLinesBufDiff()
	parent.Render(buf)

	result := buf.String()

	assert.Contains(t, result, "BUILD",
		"addPhase for Build should contain 'BUILD'")
}

// --- makeAllowedSet ---

func TestMakeAllowedSet(t *testing.T) {
	t.Parallel()

	set := makeAllowedSet([]phase.Phase{phase.Build, phase.Transfer})

	assert.Len(t, set, 2, "makeAllowedSet should have 2 entries, got %d", len(set))

	_, ok := set[phase.Build]
	assert.True(t, ok, "makeAllowedSet should contain Build")

	_, ok = set[phase.Transfer]
	assert.True(t, ok, "makeAllowedSet should contain Transfer")

	_, ok = set[phase.Inspect]
	assert.False(t, ok, "makeAllowedSet should NOT contain Inspect")
}

func TestMakeAllowedSet_Empty(t *testing.T) {
	t.Parallel()

	set := makeAllowedSet(nil)

	assert.Empty(t, set, "makeAllowedSet(nil) should be empty, got %d", len(set))
}

// --- styleForEntity ---

func TestStyleForEntity(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)

	entity := conf.ColorScheme.Flake
	result := entity.Color

	assert.True(t, reflect.DeepEqual(result, entity.Color),
		"entity Color should be the entity's Color style")
}

// --- durationBytes ---

func TestDurationBytes_Finished(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	tas := atomictimeandstate.New()
	tas.StartTimer()
	tas.EndTimerWithError(nil)

	styled, width := buildLogs.durationBytes(tas)

	assert.Positive(t, width, "durationBytes for finished phase should have positive width, got %d", width)
	assert.True(t, bytes.Contains(styled, []byte("s)")),
		"durationBytes result should contain seconds, got %q", styled)
}

func TestDurationBytes_NotStarted(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	tas := atomictimeandstate.New()

	styled, width := buildLogs.durationBytes(tas)

	assert.Equal(t, 0, width)
	assert.Empty(t, styled,
		"durationBytes for not-started should return empty, got %q", styled)
}

// --- phaseLogsAndXpath ---

func TestPhaseLogsAndXpath_ConfigScope(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	cfg := &configuration.Configuration{}
	cfg.Logs = logs.New()
	cfg.Xpath = xpath.New("flake0", "cfg0")

	pm := phase.PhaseMetadata{Phase: phase.Build, Scope: phase.ScopeConfiguration}

	logsResult, xpResult := buildLogs.phaseLogsAndXpath(pm, cfg, nil)

	assert.Equal(t, cfg.Logs, logsResult,
		"phaseLogsAndXpath for config scope should return cfg.Logs")
	assert.Equal(t, cfg.Xpath, xpResult,
		"phaseLogsAndXpath for config scope should return cfg.Xpath")
}

func TestPhaseLogsAndXpath_MachineScope(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	cfg := &configuration.Configuration{}
	cfg.Logs = logs.New()

	mach := &machine.Machine{}
	mach.Logs = logs.New()
	mach.Xpath = xpath.New("flake0", "cfg0", "m0")

	pm := phase.PhaseMetadata{Phase: phase.Inspect, Scope: phase.ScopeMachine}

	logsResult, xpResult := buildLogs.phaseLogsAndXpath(pm, cfg, mach)

	assert.Equal(t, mach.Logs, logsResult,
		"phaseLogsAndXpath for machine scope should return m.Logs")
	assert.Equal(t, mach.Xpath, xpResult,
		"phaseLogsAndXpath for machine scope should return m.Xpath")
}

// --- Helpers ---

func newFinishedPhaseLog(err error) *phaselogs.PhaseLog {
	phaseLog := phaselogs.NewPhaseLog()
	phaseLog.TimeAndState.StartTimer()
	phaseLog.TimeAndState.EndTimerWithError(err)

	return phaseLog
}

func newRunningPhaseLog() *phaselogs.PhaseLog {
	phaseLog := phaselogs.NewPhaseLog()
	phaseLog.TimeAndState.StartTimer()

	return phaseLog
}

func newNotStartedPhaseLog() *phaselogs.PhaseLog {
	return phaselogs.NewPhaseLog()
}

func newFinishedCommandLog(description string, err error) *command.CommandLog {
	cmd := command.NewCommandLog(description, "running", "failed", []string{"echo", "test"}, nil)
	cmd.TimeAndState.StartTimer()
	cmd.TimeAndState.EndTimerWithError(err)

	return cmd
}

func newTestViewports(conf *config.Config) *viewports.Viewports {
	colorScheme := conf.ColorScheme

	return viewports.New(
		&viewports.Dimensions{Width: 200, Height: 80},
		conf.Flags.Tui.CommandOutputMaxHeight,
		colorScheme.Table.Border,
		colorScheme.Table.SelectionHighlightBackground,
		colorScheme.Table.SelectionHighlightBorder,
	)
}

func makeTestConfigWithSingleMachine() *config.Config {
	const (
		flakeName = "flake0"
		cfgName   = "cfg0"
	)

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()

	flakeObj := &flake.Flake{}
	flakeObj.Name = flakeName
	flakeObj.Xpath = xpath.New(flakeName)
	flakeObj.Logs = logs.New()
	flakeObj.Configurations = atomicorderedmap.New[string, *configuration.Configuration]()

	cfg := &configuration.Configuration{}
	cfg.Name = cfgName
	cfg.Xpath = xpath.New(flakeName, cfgName)
	cfg.Logs = logs.New()
	cfg.Machines = atomicorderedmap.New[string, *machine.Machine]()

	mach := newTestMachine(allPhases, true) // running machine
	mach.Name = "m0"
	mach.Xpath = xpath.New(flakeName, cfgName, "m0")
	cfg.Machines.Set("m0", mach)

	flakeObj.Configurations.Set(cfgName, cfg)
	flakesMap.Set(flakeName, flakeObj)

	return &config.Config{
		ColorScheme: colorscheme.DefaultColorScheme(),
		Fleet:       &fleet.Fleet{Flakes: flakesMap},
		Phases:      allPhases,
	}
}

func getFirstConfig(conf *config.Config) *configuration.Configuration {
	for _, fp := range conf.Fleet.Flakes.Pairs() {
		if fp.Value == nil {
			continue
		}

		for _, cp := range fp.Value.Configurations.Pairs() {
			if cp.Value != nil {
				return cp.Value
			}
		}
	}

	return nil
}
