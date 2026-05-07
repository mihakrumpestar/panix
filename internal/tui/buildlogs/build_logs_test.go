package buildlogs

import (
	"os"
	"strings"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/tui/phaseflow"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/pkg/tui/spinners"
	"github.com/mihakrumpestar/panix/pkg/tui/tree"
	"github.com/mihakrumpestar/panix/pkg/tui/viewports"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

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
		got := formatDuration(tc.secs)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}

// --- isHideable ---

func TestIsHideable(t *testing.T) {
	t.Parallel()

	buildLogs := &BuildLogs{}

	hideablePhases := []phase.Phase{phase.Inspect, phase.Secrets}
	nonHideablePhases := []phase.Phase{phase.Build, phase.Bootstrap, phase.Transfer, phase.Activate, phase.Rollback}

	for _, p := range hideablePhases {
		if !buildLogs.isHideable(p) {
			t.Errorf("isHideable(%s) = false, want true", p)
		}
	}

	for _, p := range nonHideablePhases {
		if buildLogs.isHideable(p) {
			t.Errorf("isHideable(%s) = true, want false", p)
		}
	}
}

// --- shouldHidePhase ---

func TestShouldHidePhase_HideableFinishedNoError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(nil) // finished, no error

	if !buildLogs.shouldHidePhase(phase.Inspect, phaseLog) {
		t.Error("should hide finished hideable phase with no error")
	}
}

func TestShouldHidePhase_HideableFinishedWithError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(os.ErrNotExist) // finished, with error

	if buildLogs.shouldHidePhase(phase.Inspect, phaseLog) {
		t.Error("should NOT hide finished hideable phase with error")
	}
}

func TestShouldHidePhase_HideableRunning(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newRunningPhaseLog() // started but not finished

	if buildLogs.shouldHidePhase(phase.Inspect, phaseLog) {
		t.Error("should NOT hide running hideable phase")
	}
}

func TestShouldHidePhase_NonHideableFinishedNoError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(nil)

	if buildLogs.shouldHidePhase(phase.Build, phaseLog) {
		t.Error("should NOT hide finished non-hideable phase (Build)")
	}
}

func TestShouldHidePhase_ShowAllBuildLogs(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	conf.Flags.Tui.ShowAllBuildLogs = true
	buildLogs := New(conf, nil, nil)

	phaseLog := newFinishedPhaseLog(nil)

	if buildLogs.shouldHidePhase(phase.Inspect, phaseLog) {
		t.Error("ShowAllBuildLogs=true should NOT hide finished hideable phase")
	}
}

func TestShouldHidePhase_ShowActiveOnly(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	conf.Flags.Tui.ShowActiveOnly = true
	buildLogs := New(conf, nil, nil)

	runningPhaseLog := newRunningPhaseLog()
	if buildLogs.shouldHidePhase(phase.Build, runningPhaseLog) {
		t.Error("ShowActiveOnly should NOT hide running phase")
	}

	finishedPhaseLog := newFinishedPhaseLog(nil)
	if !buildLogs.shouldHidePhase(phase.Build, finishedPhaseLog) {
		t.Error("ShowActiveOnly should hide finished phase even if non-hideable")
	}
}

// --- layoutLine ---

func TestLayoutLine(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 80

	line := buildLogs.layoutLine(0, "left", "right", 4, 5)

	if !strings.Contains(line, "left") {
		t.Error("layoutLine should contain left text")
	}

	if !strings.Contains(line, "right") {
		t.Error("layoutLine should contain right text")
	}

	if !strings.HasSuffix(line, "right") {
		t.Error("layoutLine should end with right text")
	}
}

func TestLayoutLine_NarrowWidth(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 20

	line := buildLogs.layoutLine(6, "BUILD", "(1.23s)", 5, 7)

	if !strings.Contains(line, "BUILD") {
		t.Error("layoutLine should contain left text")
	}

	if !strings.Contains(line, "(1.23s)") {
		t.Error("layoutLine should contain right text")
	}
}

// --- spinnerOrIcon ---

func TestSpinnerOrIcon_NotStarted(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	tas := &atomictimeandstate.TimeAndState{} //nolint:exhaustruct // not started

	result := buildLogs.spinnerOrIcon(xpath.New("test"), "icon", tas)
	if result != "" {
		t.Errorf("spinnerOrIcon for not-started = %q, want empty", result)
	}
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

	result := buildLogs.spinnerOrIcon(xpath.New("test"), "OK", tasLoaded)
	if result != "OK " {
		t.Errorf("spinnerOrIcon for finished = %q, want %q", result, "OK ")
	}
}

func TestSpinnerOrIcon_Running(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	tas := atomictimeandstate.New()
	tas.StartTimer()
	tasLoaded := tas.Load()

	result := buildLogs.spinnerOrIcon(xpath.New("test"), "OK", tasLoaded)
	if result == "" {
		t.Error("spinnerOrIcon for running should return spinner, got empty")
	}

	if result == "OK " {
		t.Error("spinnerOrIcon for running should NOT return finished icon")
	}
}

// --- addPhases ---

func TestAddPhases_NilLogNode(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	parent := tree.New().Root("parent")

	result := buildLogs.addPhases(parent, nil, xpath.New("test"), 0, false, phase.Build)
	if result {
		t.Error("addPhases with nil logNode should return false")
	}

	if parent.Length() != 0 {
		t.Error("addPhases with nil logNode should not add children")
	}
}

func TestAddPhases_NilPhaseLogs(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	logNode := &logs.Logs{} //nolint:exhaustruct // PhaseLogs is nil

	parent := tree.New().Root("parent")

	result := buildLogs.addPhases(parent, logNode, xpath.New("test"), 0, false, phase.Build)
	if result {
		t.Error("addPhases with nil PhaseLogs should return false")
	}
}

func TestAddPhasesSingle_HideableFinishedFiltered(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	logNode := logs.New()
	phaseLog := newFinishedPhaseLog(nil) // finished, no error
	logNode.PhaseLogs.Set(phase.Inspect, phaseLog)

	parent := tree.New().Root("parent")

	result := buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Inspect)
	if result {
		t.Error("addPhasesSingle should return false when phase is hidden")
	}

	if parent.Length() != 0 {
		t.Errorf("addPhasesSingle should add 0 children when phase hidden, got %d", parent.Length())
	}
}

func TestAddPhasesSingle_HideableFinishedWithErrorNotFiltered(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	logNode := logs.New()
	phaseLog := newFinishedPhaseLog(os.ErrNotExist) // finished with error
	logNode.PhaseLogs.Set(phase.Inspect, phaseLog)

	parent := tree.New().Root("parent")

	// stopAtError=false: addPhases returns false regardless, but the child should still be added
	buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Inspect)

	if parent.Length() != 1 {
		t.Errorf("addPhasesSingle should add child when phase has error, got %d", parent.Length())
	}

	// With stopAtError=true, addPhases should return true
	parent2 := tree.New().Root("parent")
	result2 := buildLogs.addPhases(parent2, logNode, xpath.New("test"), 6, true, phase.Inspect)

	if !result2 {
		t.Error("addPhasesSingle with stopAtError should return true when phase has error")
	}
}

func TestAddPhasesSingle_BuildPhaseNotFiltered(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	logNode := logs.New()
	phaseLog := newFinishedPhaseLog(nil) // finished, no error, but NOT hideable
	logNode.PhaseLogs.Set(phase.Build, phaseLog)

	parent := tree.New().Root("parent")

	result := buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Build)
	if result {
		t.Error("addPhasesSingle should return false when phase finishes with no error")
	}

	if parent.Length() != 1 {
		t.Errorf("addPhasesSingle should add child for non-hideable phase, got %d", parent.Length())
	}
}

func TestAddPhasesMulti_Filtering(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	logNode := logs.New()

	// Inspect: finished, no error → should be hidden
	inspectLog := newFinishedPhaseLog(nil)
	logNode.PhaseLogs.Set(phase.Inspect, inspectLog)

	// Build: finished, no error → NOT hideable, should be shown
	buildLog := newFinishedPhaseLog(nil)
	logNode.PhaseLogs.Set(phase.Build, buildLog)

	parent := tree.New().Root("parent")

	buildLogs.addPhases(parent, logNode, xpath.New("test"), 6, false, phase.Inspect, phase.Build)

	if parent.Length() != 1 {
		t.Errorf("addPhasesMulti should add 1 child (Build only, Inspect hidden), got %d", parent.Length())
	}
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

	phaseNode := tree.New().Root("phase")

	hasError := buildLogs.addCommands(phaseNode, phaseLog, phase.Build, xpath.New("test"), 6)

	if hasError {
		t.Error("addCommands with nil CommandLogs and no error should return false")
	}
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
	cmd.Output.WriteLineString("error output")

	phaseNode := tree.New().Root("phase")

	hasError := buildLogs.addCommands(phaseNode, phaseLog, phase.Build, xpath.New("test"), 6)

	if !hasError {
		t.Error("addCommands should return true when command has error")
	}

	if phaseNode.Length() == 0 {
		t.Error("addCommands should add command children")
	}
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

	phaseNode := tree.New().Root("phase")

	buildLogs.addCommands(phaseNode, phaseLog, phase.Inspect, xpath.New("test"), 6)

	if phaseNode.Length() != 1 {
		t.Errorf("addCommands for hideable phase should show only last command, got %d children", phaseNode.Length())
	}
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

			phaseNode := tree.New().Root("phase")

			buildLogs.addCommands(phaseNode, phaseLog, testCase.phaseI, xpath.New("test"), 6)

			if phaseNode.Length() != testCase.wantCmds {
				t.Errorf("got %d children, want %d", phaseNode.Length(), testCase.wantCmds)
			}
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
	cmd.Output.WriteLineString("build output line 1")
	cmd.Output.WriteLineString("build output line 2")

	cmdNode := tree.New().Root("cmd")
	tas := cmd.TimeAndState.Load()

	buildLogs.addCommandChildren(cmdNode, cmd, xpath.New("test"), tas, 9)

	if cmdNode.Length() < 2 {
		t.Errorf("addCommandChildren should add output and error children, got %d", cmdNode.Length())
	}
}

func TestAddCommandChildren_NoOutputNoError(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.viewports = newTestViewports(conf)

	cmd := newFinishedCommandLog("test cmd", nil) // no error

	cmdNode := tree.New().Root("cmd")
	tas := cmd.TimeAndState.Load()

	buildLogs.addCommandChildren(cmdNode, cmd, xpath.New("test"), tas, 9)

	if cmdNode.Length() != 0 {
		t.Errorf("addCommandChildren with no output and no error should add 0 children, got %d", cmdNode.Length())
	}
}

// --- entityNode ---

func TestEntityNode(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.styledTreeLine = conf.ColorScheme.Tree.Enumerator.Render("│")

	logNode := logs.New()
	node := buildLogs.entityNode(0, conf.ColorScheme.Flake, "my-flake", logNode, true)

	var buf []byte
	node.View(&buf)

	result := string(buf)

	if !strings.Contains(result, "my-flake") {
		t.Error("entityNode should contain the name")
	}

	if !strings.Contains(result, "(0.00s)") {
		t.Error("entityNode should contain duration")
	}
}

func TestEntityNode_NilLogNode(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.styledTreeLine = conf.ColorScheme.Tree.Enumerator.Render("│")

	node := buildLogs.entityNode(0, conf.ColorScheme.Flake, "my-flake", nil, true)

	var buf []byte
	node.View(&buf)

	result := string(buf)

	if !strings.Contains(result, "my-flake") {
		t.Error("entityNode with nil logNode should still contain the name")
	}

	if !strings.Contains(result, "(0.00s)") {
		t.Error("entityNode with nil logNode should show 0.00s duration")
	}
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

	cfgNode := tree.New().Root("cfg")

	buildLogs.buildDefaultTree(cfgNode, cfg)

	if cfgNode.Length() == 0 {
		t.Error("buildDefaultTree should produce children")
	}
}

func TestBuildDefaultTree_EmptyMachines(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(1, 1, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	cfg := getFirstConfig(conf)

	// Add Build phase log to the config (config-scoped)
	cfg.Logs.PhaseLogs.Set(phase.Build, newFinishedPhaseLog(nil))

	cfgNode := tree.New().Root("cfg")

	buildLogs.buildDefaultTree(cfgNode, cfg)

	if cfgNode.Length() == 0 {
		t.Error("buildDefaultTree should produce config-scoped phases even with no machines")
	}
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

	cfgNode := tree.New().Root("cfg")

	buildLogs.buildPhaseSelectedTree(cfgNode, cfg)

	if cfgNode.Length() == 0 {
		t.Error("buildPhaseSelectedTree for config-scoped phase should add children")
	}
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

	cfgNode := tree.New().Root("cfg")

	buildLogs.buildPhaseSelectedTree(cfgNode, cfg)
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

	result := buildLogs.View(vp, sp)

	stripped := stripANSI(result)

	if !strings.Contains(stripped, "Build Logs") {
		t.Error("View should contain 'Build Logs' header")
	}

	if !strings.Contains(stripped, "flake0") {
		t.Error("View should contain flake name")
	}
}

func TestView_EmptyFleet(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := buildLogs.View(vp, sp)

	if !strings.Contains(result, "Build Logs") {
		t.Error("View should contain 'Build Logs' header even with empty fleet")
	}
}

func TestView_NilFlake(t *testing.T) {
	t.Parallel()

	conf := makeTestConfigWithSingleMachine()
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	// Add a nil flake entry — View should skip it
	conf.Fleet.Flakes.Set("nil-flake", nil)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := buildLogs.View(vp, sp)

	if !strings.Contains(stripANSI(result), "Build Logs") {
		t.Error("View should contain 'Build Logs' header")
	}
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

	result := buildLogs.View(vp, sp)

	if !strings.Contains(stripANSI(result), "Build Logs") {
		t.Error("View should contain 'Build Logs' header")
	}
}

func TestView_MultipleFlakes(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(2, 2, 2, nil)
	st := statstable.New(conf.Fleet, conf.ColorScheme)
	ps := phaseflow.New(conf.Fleet, conf.ColorScheme, conf.Phases)
	buildLogs := New(conf, st, ps)

	vp := newTestViewports(conf)
	sp := spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)

	result := buildLogs.View(vp, sp)

	if !strings.Contains(result, "flake0") {
		t.Error("View should contain first flake name")
	}

	if !strings.Contains(result, "flake1") {
		t.Error("View should contain second flake name")
	}
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

	parent := tree.New().Root("parent")

	result := buildLogs.addPhase(parent, xpath.New("test"), phase.Build, phaseLog, 6)

	if result {
		t.Error("addPhase for not-started phase should return false (no error)")
	}

	if parent.Length() != 1 {
		t.Errorf("addPhase should add the phase even when not started, got %d children", parent.Length())
	}
}

func TestAddPhase_RunningPhaseShowsSpinner(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)
	buildLogs.contentWidth = 120
	buildLogs.spinners = spinners.New(conf.ColorScheme.Spinner.Frames, conf.ColorScheme.Spinner.Interval)
	buildLogs.viewports = newTestViewports(conf)

	phaseLog := newRunningPhaseLog()

	parent := tree.New().Root("parent")

	buildLogs.addPhase(parent, xpath.New("test"), phase.Build, phaseLog, 6)

	if parent.Length() != 1 {
		t.Error("addPhase for running phase should add child")
	}

	var buf []byte
	parent.View(&buf)

	result := string(buf)

	if !strings.Contains(result, "BUILD") {
		t.Error("addPhase for Build should contain 'BUILD'")
	}
}

// --- makeAllowedSet ---

func TestMakeAllowedSet(t *testing.T) {
	t.Parallel()

	set := makeAllowedSet([]phase.Phase{phase.Build, phase.Transfer})

	if len(set) != 2 {
		t.Fatalf("makeAllowedSet should have 2 entries, got %d", len(set))
	}

	_, ok := set[phase.Build]
	if !ok {
		t.Error("makeAllowedSet should contain Build")
	}

	_, ok = set[phase.Transfer]
	if !ok {
		t.Error("makeAllowedSet should contain Transfer")
	}

	_, ok = set[phase.Inspect]
	if ok {
		t.Error("makeAllowedSet should NOT contain Inspect")
	}
}

func TestMakeAllowedSet_Empty(t *testing.T) {
	t.Parallel()

	set := makeAllowedSet(nil)

	if len(set) != 0 {
		t.Errorf("makeAllowedSet(nil) should be empty, got %d", len(set))
	}
}

// --- styleForEntity ---

func TestStyleForEntity(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	entity := conf.ColorScheme.Flake
	result := buildLogs.styleForEntity(entity)

	if result != entity.Color {
		t.Error("styleForEntity should return the entity's Color style")
	}
}

// --- durationText ---

func TestDurationText_Finished(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	tas := atomictimeandstate.New()
	tas.StartTimer()
	tas.EndTimerWithError(nil)

	styled, width := buildLogs.durationText(conf.ColorScheme.Phase.Color, tas)

	if width <= 0 {
		t.Errorf("durationText for finished phase should have positive width, got %d", width)
	}

	if !strings.Contains(styled, "s)") {
		t.Errorf("durationText result should contain seconds, got %q", styled)
	}
}

func TestDurationText_NotStarted(t *testing.T) {
	t.Parallel()

	conf := makeTestConfig(0, 0, 0, nil)
	buildLogs := New(conf, nil, nil)

	tas := atomictimeandstate.New()

	styled, width := buildLogs.durationText(conf.ColorScheme.Phase.Color, tas)

	if width != 0 {
		t.Errorf("durationText for not-started should have 0 width, got %d", width)
	}

	if styled != "" {
		t.Errorf("durationText for not-started should return empty string, got %q", styled)
	}
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

	if logsResult != cfg.Logs {
		t.Error("phaseLogsAndXpath for config scope should return cfg.Logs")
	}

	if xpResult != cfg.Xpath {
		t.Error("phaseLogsAndXpath for config scope should return cfg.Xpath")
	}
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

	if logsResult != mach.Logs {
		t.Error("phaseLogsAndXpath for machine scope should return m.Logs")
	}

	if xpResult != mach.Xpath {
		t.Error("phaseLogsAndXpath for machine scope should return m.Xpath")
	}
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

func stripANSI(s string) string {
	var buf strings.Builder

	inEsc := false

	for _, char := range s {
		if char == 0x1b {
			inEsc = true

			continue
		}

		if inEsc && char == 'm' {
			inEsc = false

			continue
		}

		if inEsc {
			continue
		}

		buf.WriteRune(char)
	}

	return buf.String()
}
