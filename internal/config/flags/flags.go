package flags

import (
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/mattn/go-isatty"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
)

type ActivationMode string

const (
	ActivationModeCheck       ActivationMode = "check"        // run pre-switch checks and exit
	ActivationModeSwitch      ActivationMode = "switch"       // make the configuration the boot default and activate now
	ActivationModeBoot        ActivationMode = "boot"         // make the configuration the boot default
	ActivationModeTest        ActivationMode = "test"         // activate the configuration, but don't make it the boot default
	ActivationModeDryActivate ActivationMode = "dry-activate" // show what would be done if this configuration were activated
)

type OutputMode string

const (
	OutputModeTui     OutputMode = "tui"     // interactive TUI
	OutputModeConsole OutputMode = "console" // human-readable log output to stdout
	OutputModeJSON    OutputMode = "json"    // JSON log output to stdout
)

type ConfigFlag struct {
	Config string `yaml:"config" json:"config" short:"c" help:"Config file" default:"panix.yml"`
}

type WorkflowFlags struct {
	ConfigFlag           `yaml:",inline"`
	Tags                 []string       `yaml:"tags" short:"t" help:"Filter machines by tags (flakes, configs and names are already registered as tags)"`
	Bootstrap            Bootstrap      `yaml:"bootstrap" embed:"" prefix:"bootstrap."`
	RequireAllSuccess    bool           `yaml:"require_all_success" help:"Abort if any task fails, primarily for CI/CD"`
	OverrideLocalMachine string         `yaml:"override_local_machine" help:"Hostname of the machine that is local (won't use ssh to connect to it)"`
	DryRun               bool           `yaml:"dry_run" help:"Show what would be done without executing"`
	DryRunWithInspect    bool           `yaml:"dry_run_with_inspect" help:"Show what would be done without executing, but with real inspect query"`
	Timeout              time.Duration  `yaml:"timeout" help:"Timeout per command (eg. '1h', '1m15s')" default:"2h"`
	SkipPhases           []phase.Phase  `yaml:"skip_phases" short:"s" help:"Declare phases to skip (not all phases can be skipped)"`
	ExitOnComplete       bool           `yaml:"exit_on_complete" help:"Exit TUI on completion; 'retry' and 'restart' are disabled in this mode"`
	ActivationMode       ActivationMode `yaml:"activation_mode" help:"Activation mode: check, switch, boot, test, dry-activate" default:"switch" validate:"omitempty,oneof=check switch boot test dry-activate"` //nolint:lll
	Output               OutputMode     `yaml:"output" help:"Output mode: tui, console, json" default:"tui" validate:"omitempty,oneof=tui console json"`                                                         //nolint:lll

	Tui      `yaml:"tui" embed:"" prefix:"tui."`           //nolint:embeddedstructfieldcheck
	Snapshot `yaml:"snapshot" embed:"" prefix:"snapshot."` //nolint:embeddedstructfieldcheck
	Logging  `yaml:"logging"`                              //nolint:embeddedstructfieldcheck
}

type RollbackFlags struct {
	Generation int `name:"gen" yaml:"rollback_generation" json:"rollback_generation" help:"0=current generation, -N=Nth before current, N=specific generation" default:"-1"`
}

type Flags struct {
	WorkflowFlags `yaml:",inline"`
	RollbackFlags `yaml:",inline"`
}

type Bootstrap struct {
	DisableDisko bool `yaml:"disable_disko" help:"Disables building, transfer and execution of disko tool"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"show_all_build_logs" help:"Show all build logs in TUI (keybind h)"`
	ShowActiveOnly         bool `yaml:"show_active_only" help:"Show only running or errored logs in TUI build logs (keybind a)"`
	ShowCommandsInLabels   bool `yaml:"show_commands_in_labels" help:"Show raw commands instead of descriptions as labels in build logs (keybind c)"`
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" help:"Maximum height for command labels and outputs viewports in TUI" default:"8"`
}

type Snapshot struct {
	Dir     string `yaml:"snapshot_dir" help:"Directory to save snapshots" default:"."`
	OnRetry bool   `yaml:"snapshot_on_retry" help:"Take snapshot before retry"`
	OnExit  bool   `yaml:"snapshot_on_exit" help:"Take snapshot on exit"`
}

type Logging struct {
	Log        bool   `yaml:"log" short:"l" help:"Enable logging to file"`
	LogFile    string `yaml:"log_file" help:"Log file path (epoch timestamp appended before .log)" default:"panix.log"`
	Debug      bool   `yaml:"debug" short:"d" help:"Debug output (enables logging)"`
	CPUProfile string `yaml:"cpu_profile" help:"Path for cpu profiling to file, declaring it enables it"`
}

func (f *Flags) DefautlIfNoTTY() {
	if f.Output == "" {
		if !IsTerminal() {
			f.Output = OutputModeConsole
		} else {
			f.Output = OutputModeTui
		}
	}

	if f.Output != OutputModeTui {
		f.ExitOnComplete = true
	}
}

func (f *Flags) MergeConfWithCliFlags(cli Flags) error {
	err := mergo.Merge(f, cli)
	if err != nil {
		return errors.Wrap(err, "failed to merge flags")
	}

	return nil
}

// Helpers

func IsTerminal() bool {
	fd := os.Stdout.Fd()

	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
