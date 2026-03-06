package flags

import (
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
	"github.com/pkg/errors"
)

const (
	defaultTimeout              = 2 * time.Hour
	defaultCommandOutputMaxSize = 8
)

type Flags struct {
	Config               string         `yaml:"config" short:"c" help:"Config file" default:"panix.yml"`
	Tags                 []string       `yaml:"tags" short:"t" help:"Filter machines by tags (flakes, configs and names are already registered as tags)"`
	Bootstrap            Bootstrap      `yaml:"bootstrap" embed:"" prefix:"bootstrap."`
	RequireAllSuccess    bool           `yaml:"require_all_success" help:"Abort if any task fails, primarily for CI/CD"`
	OverrideLocalMachine string         `yaml:"override_local_machine" help:"Hostname of the machine that is local (won't use ssh to connect to it)"`
	DryRun               bool           `yaml:"dry_run" help:"Show what would be done without executing"`
	DryRunWithInspect    bool           `yaml:"dry_run_with_inspect" help:"Show what would be done without executing, but with real inspect query"`
	Timeout              time.Duration  `yaml:"timeout" help:"Timeout for workflow (eg. '1h', '1m15s')" default:"2h"`
	SkipPhases           []phases.Phase `yaml:"skip_phases" short:"s" help:"Declare phases to skip (not all phases can be skipped)"`
	ExitOnComplete       bool           `yaml:"exit_on_complete" help:"Exit TUI on completion; 'retry' and 'restart' are disabled in this mode"`

	Tui     `yaml:"tui" embed:"" prefix:"tui."`
	Logging `yaml:"logging"`
}

type Bootstrap struct {
	Only         bool `yaml:"only" help:"Only initializes uninitialized machines"`
	DisableAuto  bool `yaml:"disable_auto" help:"Disable automatic bootstrap (even if target machine does not have NixOS installed)"`
	DisableDisko bool `yaml:"disable_disko" help:"Disables building, transfer and bootstrap of disko tool"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"show_all_build_logs" help:"Show all build logs in TUI (keybind h)"`
	ShowActiveOnly         bool `yaml:"show_active_only" help:"Show only running or errored logs in TUI build logs (keybind a)"`
	ShowCommandsInLabels   bool `yaml:"show_commands_in_labels" help:"Show raw commands instead of descriptions as labels in build logs (keybind c)"`
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" help:"Maximum height for command labels and outputs viewports in TUI" default:"8"`
}

type Logging struct {
	Log        bool   `yaml:"log" short:"l" help:"Enable logging to file"`
	LogFile    string `yaml:"log_file" help:"Log file path" default:"panix.log"`
	Debug      bool   `yaml:"debug" short:"d" help:"Debug output (enables logging)"`
	CPUProfile string `yaml:"cpu_profile" help:"Path for cpu profiling to file, declaring it enables it"`
}

func (f *Flags) SetDefault(reverse bool) {
	defaultHostname, _ := os.Hostname()

	toggle(reverse, &f.Config, "panix.yml", "")
	toggle(reverse, &f.OverrideLocalMachine, defaultHostname, "")
	toggle(reverse, &f.Timeout, defaultTimeout, 0)
	toggle(reverse, &f.Tui.CommandOutputMaxHeight, defaultCommandOutputMaxSize, 0)
	toggle(reverse, &f.LogFile, "panix.log", "")
}

func (f *Flags) MergeConfWithCliFlags(cli Flags) error {
	f.SetDefault(true)

	if err := mergo.Merge(f, cli); err != nil {
		return errors.Wrap(err, "failed to merge flags")
	}

	f.SetDefault(false)

	return nil
}

func toggle[T comparable](reverse bool, ptr *T, def, zero T) {
	if reverse {
		if *ptr == def {
			*ptr = zero
		}
	} else {
		if *ptr == zero {
			*ptr = def
		}
	}
}
