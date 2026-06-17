package flags

import (
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/mattn/go-isatty"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/profile"
	"github.com/pkg/errors"
)

type OutputMode string

const (
	OutputModeTui     OutputMode = "tui"     // interactive TUI
	OutputModeConsole OutputMode = "console" // human-readable log output to stdout
	OutputModeJSON    OutputMode = "json"    // JSON log output to stdout
)

type ConfigFlags struct {
	Config string `yaml:"-" json:"config" short:"c" help:"Config file" validate:"required,filepath" default:"panix.yml"`
}

//nolint:lll
type WorkflowFlags struct {
	ConfigFlags `yaml:",inline"`
	EvalFlags   `yaml:",inline"`

	Output OutputMode `yaml:"output" json:"output" help:"Output mode: tui, console, json" default:"tui" validate:"omitempty,oneof=tui console json"`

	RequireAllSuccess bool `yaml:"require_all_success" json:"require_all_success,omitempty" help:"Abort if any task fails, primarily for CI/CD"`
	ExitOnComplete    bool `yaml:"exit_on_complete" json:"exit_on_complete,omitempty" help:"Exit TUI on completion ('retry' and 'restart' are disabled in this mode)"`

	LocalMachineHostname string `yaml:"local_machine_hostname" json:"local_machine_hostname,omitempty" help:"Hostname of the machine that is local (won't use ssh to connect to it) (default: your deployment machine hostname)"`
	DryRun               bool   `yaml:"dry_run" json:"dry_run,omitempty" help:"Show what would be done without executing"`
	DryRunWithInspect    bool   `yaml:"dry_run_with_inspect" json:"dry_run_with_inspect,omitempty" help:"Show what would be done without executing, but with real inspect query"`

	Logging         `yaml:"logging" json:"logging"` //nolint:embeddedstructfieldcheck
	Snapshot        `yaml:"snapshot" json:"snapshot" embed:"" prefix:"snapshot."`
	Tui             `yaml:"tui" json:"tui" embed:"" prefix:"tui."`
	profile.Profile `yaml:"profile" json:"profile" embed:"" prefix:"profile."`
}

//nolint:lll
type EvalFlags struct {
	ValidateFlags `yaml:",inline"`

	Tags           []string                  `yaml:"tags" json:"tags,omitempty" short:"t" help:"Filter machines by tags (flakes, configurations and machine names are already registered as tags)"`
	SkipPhases     []phase.Phase             `yaml:"skip_phases" json:"skip_phases,omitempty" short:"s" help:"Declare phases to skip (not all phases can be skipped)"`
	Timeout        time.Duration             `yaml:"timeout" json:"timeout,omitempty" help:"Timeout per command (eg. '1h', '1m15s')" default:"2h"`
	ActivationMode attributes.ActivationMode `yaml:"activation_mode" json:"activation_mode,omitempty" help:"Activation mode: check, switch, boot, test, dry-activate (overrides machine specific ones)" validate:"omitempty,oneof=check switch boot test dry-activate"`
}

//nolint:lll
type RollbackFlags struct {
	Generation int `name:"gen" yaml:"rollback_generation" json:"rollback_generation,omitempty" help:"0=current generation, -N=Nth before current, N=specific generation" default:"-1"`
}

type Flags struct {
	WorkflowFlags `yaml:",inline"`
	RollbackFlags `yaml:",inline"`
}

type ValidateFlags struct {
	Validate Validate `yaml:"validate" json:"validate" embed:"" prefix:"validate." help:"Validator settings"`
}

//nolint:lll
type Validate struct {
	Flakes           bool `yaml:"flakes" json:"flakes,omitempty" help:"Validate flake URLs and configuration keys"`
	BootstrapSecrets bool `yaml:"bootstrap_secrets" json:"bootstrap_secrets,omitempty" help:"Validate that bootstrap disk encryption key local paths exist on disk"`
}

//nolint:lll
type Tui struct {
	ShowAllBuildLogs       bool `yaml:"show_all_build_logs" json:"show_all_build_logs,omitempty" help:"Show all build logs in TUI (keybind h)"`
	ShowActiveOnly         bool `yaml:"show_active_only" json:"show_active_only,omitempty" help:"Show only running or errored logs in TUI build logs (keybind a)"`
	ShowCommandsInLabels   bool `yaml:"show_commands_in_labels" json:"show_commands_in_labels,omitempty" help:"Show raw commands instead of descriptions as labels in build logs (keybind c)"`
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" json:"command_output_max_height" help:"Maximum height for command labels and outputs viewports in TUI" default:"8"`
}

type Snapshot struct {
	Dir     string `yaml:"dir" json:"dir" help:"Directory to save snapshots" validate:"omitempty,dir,dir_exists" default:"."`
	OnRetry bool   `yaml:"on_retry" json:"on_retry,omitempty" help:"Take snapshot before retry"`
	OnExit  bool   `yaml:"on_exit" json:"on_exit,omitempty" help:"Take snapshot on exit"`
}

//nolint:lll
type Logging struct {
	Log     bool   `yaml:"log" json:"log,omitempty" short:"l" help:"Enable logging to file"`
	LogFile string `yaml:"log_file" json:"log_file,omitempty" help:"Log file path (epoch timestamp appended before .log)" validate:"omitempty,filepath" default:"panix.log"`
	Debug   bool   `yaml:"debug" json:"debug,omitempty" short:"d" help:"Debug mode (enables logging)"`
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
