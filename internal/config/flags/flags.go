package flags

import (
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/mattn/go-isatty"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
	"github.com/pkg/errors"
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
	Tags                 []string                  `yaml:"tags" json:"tags,omitempty" short:"t" help:"Filter machines by tags (flakes, configs and names are already registered as tags)"`
	RequireAllSuccess    bool                      `yaml:"require_all_success" json:"require_all_success,omitempty" help:"Abort if any task fails, primarily for CI/CD"`
	LocalMachineHostname string                    `yaml:"local_machine_hostname" json:"local_machine_hostname,omitempty" help:"Hostname of the machine that is local (won't use ssh to connect to it); default:your machines hostname"`
	DryRun               bool                      `yaml:"dry_run" json:"dry_run,omitempty" help:"Show what would be done without executing"`
	DryRunWithInspect    bool                      `yaml:"dry_run_with_inspect" json:"dry_run_with_inspect,omitempty" help:"Show what would be done without executing, but with real inspect query"`
	Timeout              time.Duration             `yaml:"timeout" json:"timeout,omitempty" help:"Timeout per command (eg. '1h', '1m15s')" default:"2h"`
	SkipPhases           []phase.Phase             `yaml:"skip_phases" json:"skip_phases,omitempty" short:"s" help:"Declare phases to skip (not all phases can be skipped)"`
	ExitOnComplete       bool                      `yaml:"exit_on_complete" json:"exit_on_complete,omitempty" help:"Exit TUI on completion; 'retry' and 'restart' are disabled in this mode"`
	ActivationMode       attributes.ActivationMode `yaml:"activation_mode" json:"activation_mode,omitempty" help:"Activation mode: check, switch, boot, test, dry-activate; overrides machine one" validate:"omitempty,oneof=check switch boot test dry-activate"` //nolint:lll
	Output               OutputMode                `yaml:"output" json:"output" help:"Output mode: tui, console, json" default:"tui" validate:"omitempty,oneof=tui console json"`                                                                                  //nolint:lll

	Tui      `yaml:"tui" json:"tui" embed:"" prefix:"tui."`                //nolint:embeddedstructfieldcheck
	Snapshot `yaml:"snapshot" json:"snapshot" embed:"" prefix:"snapshot."` //nolint:embeddedstructfieldcheck
	Logging  `yaml:"logging" json:"logging"`                               //nolint:embeddedstructfieldcheck
}

type RollbackFlags struct {
	Generation int `name:"gen" yaml:"rollback_generation" json:"rollback_generation,omitempty" help:"0=current generation, -N=Nth before current, N=specific generation" default:"-1"`
}

type Flags struct {
	WorkflowFlags `yaml:",inline"`
	RollbackFlags `yaml:",inline"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"show_all_build_logs" json:"show_all_build_logs,omitempty" help:"Show all build logs in TUI (keybind h)"`
	ShowActiveOnly         bool `yaml:"show_active_only" json:"show_active_only,omitempty" help:"Show only running or errored logs in TUI build logs (keybind a)"`
	ShowCommandsInLabels   bool `yaml:"show_commands_in_labels" json:"show_commands_in_labels,omitempty" help:"Show raw commands instead of descriptions as labels in build logs (keybind c)"`
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" json:"command_output_max_height" help:"Maximum height for command labels and outputs viewports in TUI" default:"8"`
}

type Snapshot struct {
	Dir     string `yaml:"dir" json:"dir" help:"Directory to save snapshots" default:"."`
	OnRetry bool   `yaml:"on_retry" json:"on_retry,omitempty" help:"Take snapshot before retry"`
	OnExit  bool   `yaml:"on_exit" json:"on_exit,omitempty" help:"Take snapshot on exit"`
}

type Logging struct {
	Log        bool   `yaml:"log" json:"log,omitempty" short:"l" help:"Enable logging to file"`
	LogFile    string `yaml:"log_file" json:"log_file,omitempty" help:"Log file path (epoch timestamp appended before .log)" default:"panix.log"`
	Debug      bool   `yaml:"debug" json:"debug,omitempty" short:"d" help:"Debug output (enables logging)"`
	CPUProfile string `yaml:"cpu_profile" json:"cpu_profile,omitempty" help:"Path for cpu profiling to file, declaring it enables it"`
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
