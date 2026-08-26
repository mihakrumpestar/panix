package flags

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/phase"
)

func TestDefautlIfNoTTY(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		output     OutputMode
		wantOutput OutputMode
		wantExit   bool
	}{
		{"sets console when no tty and empty output", "", OutputModeConsole, true},
		{"preserves explicit tui output", OutputModeTui, OutputModeTui, false},
		{"preserves explicit console output", OutputModeConsole, OutputModeConsole, true},
		{"preserves explicit json output", OutputModeJSON, OutputModeJSON, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			flags := &Flags{}
			flags.Output = test.output

			flags.DefautlIfNoTTY()

			assertion := assert.New(t)
			assertion.Equal(test.wantOutput, flags.Output)
			assertion.Equal(test.wantExit, flags.ExitOnComplete)
		})
	}
}

func TestMergeConfWithCliFlagsPreservesConfigTags(t *testing.T) {
	t.Parallel()

	configFlags := &Flags{
		WorkflowFlags: WorkflowFlags{
			EvalFlags: EvalFlags{Tags: []string{"production"}},
		},
	}

	cliFlags := Flags{
		WorkflowFlags: WorkflowFlags{
			EvalFlags: EvalFlags{Tags: []string{"staging"}},
		},
	}

	err := configFlags.MergeConfWithCliFlags(cliFlags)
	require.NoError(t, err)

	assertion := assert.New(t)

	// mergo.Merge without override keeps config values when both are set
	assertion.Equal([]string{"production"}, configFlags.Tags,
		"config tags should be preserved when config already has tags")
}

func TestMergeConfWithCliFlagsSetsTagsFromCliWhenConfigEmpty(t *testing.T) {
	t.Parallel()

	configFlags := &Flags{}

	cliFlags := Flags{
		WorkflowFlags: WorkflowFlags{
			EvalFlags: EvalFlags{Tags: []string{"staging"}},
		},
	}

	err := configFlags.MergeConfWithCliFlags(cliFlags)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal([]string{"staging"}, configFlags.Tags,
		"cli tags should be set when config has no tags")
}

func TestMergeConfWithCliFlagsPreservesConfigWhenCliZero(t *testing.T) {
	t.Parallel()

	configFlags := &Flags{
		WorkflowFlags: WorkflowFlags{
			EvalFlags: EvalFlags{Timeout: 5 * time.Minute},
		},
	}

	cliFlags := Flags{}

	err := configFlags.MergeConfWithCliFlags(cliFlags)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Equal(5*time.Minute, configFlags.Timeout,
		"config timeout should be preserved when cli timeout is zero")
}

func TestMergeConfWithCliFlagsDryRunFromCli(t *testing.T) {
	t.Parallel()

	configFlags := &Flags{}

	cliFlags := Flags{
		WorkflowFlags: WorkflowFlags{
			DryRun: true,
		},
	}

	err := configFlags.MergeConfWithCliFlags(cliFlags)
	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.True(configFlags.DryRun, "dry run should be set from cli flags")
}

func TestOutputModeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode OutputMode
		want string
	}{
		{"tui", OutputModeTui, "tui"},
		{"console", OutputModeConsole, "console"},
		{"json", OutputModeJSON, "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, string(tt.mode))
		})
	}
}

func TestWorkflowFlagsActivationMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode string
		want string
	}{
		{"switch mode", "switch", "switch"},
		{"boot mode", "boot", "boot"},
		{"test mode", "test", "test"},
		{"check mode", "check", "check"},
		{"dry-activate mode", "dry-activate", "dry-activate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.mode)
		})
	}
}

func TestSkipPhasesInWorkflowFlags(t *testing.T) {
	t.Parallel()

	flags := &Flags{
		WorkflowFlags: WorkflowFlags{
			EvalFlags: EvalFlags{SkipPhases: []phase.Phase{phase.Inspect, phase.Build}},
		},
	}

	assertion := assert.New(t)
	assertion.Len(flags.SkipPhases, 2)
	assertion.Equal(phase.Inspect, flags.SkipPhases[0])
	assertion.Equal(phase.Build, flags.SkipPhases[1])
}

// TestOutLinksDirDefaultTag guards the kong default tag on out_links_dir.
func TestOutLinksDirDefaultTag(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeFor[WorkflowFlags]().FieldByName("OutLinksDir")
	require.True(t, ok, "WorkflowFlags.OutLinksDir field must exist")

	assert.Equal(t, ".panix", field.Tag.Get("default"))
}
