package config_flags

import (
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type Flags struct {
	Config               string         `yaml:"config" short:"c" help:"config file" default:"panix.yml"`
	Tags                 []string       `yaml:"tags" short:"t" help:"filter machines by tags (flakes, configurations and machine names are already registered as tags, children inherit all parent tags)"`
	Bootstrap            Bootstrap      `yaml:"bootstrap" embed:"" prefix:"bootstrap."`
	RequireAllSuccess    bool           `yaml:"require_all_success" help:"abort & rollback if any task fails, primarily for CI/CD"`
	OverrideLocalMachine string         `yaml:"override_local_machine" help:"hostname of the machine that is local (won't use ssh to connect to it)"`
	DryRun               bool           `yaml:"dry_run" help:"show what would be done without executing"`
	DryRunWithStatus     bool           `yaml:"dry_run_with_status" help:"show what would be done without executing, but with real status query"`
	Timeout              time.Duration  `yaml:"timeout" help:"timeout for workflow" default:"2h"`
	SkipPhases           []phases.Phase `yaml:"skip_phases" short:"s" help:"declare phases to skip"`
	ExitOnComplete       bool           `yaml:"exit_on_complete" help:"exit TUI immediately when workflow completes (otherwise stays open until user quits)"`

	Tui     `yaml:"tui" embed:"" prefix:"tui."`
	Logging `yaml:"logging"`
}

type Bootstrap struct {
	Only         bool `yaml:"only" help:"only initializes uninitialized machines"`
	DisableAuto  bool `yaml:"disable_auto" help:"disable automatic bootstrap (even if target machine does not have NixOS installed)"`
	DisableDisko bool `yaml:"disable_disko" help:"disables building, transfer and bootstrap of disko tool"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"show_all_build_logs" help:"show all build logs in TUI"`
	ShowActiveOnly         bool `yaml:"show_active_only" help:"show only running or errored logs in TUI build logs"`
	ShowCommandsInLabels   bool `yaml:"show_commands_in_labels" help:"show commands instead of descriptions as labels in build logs"`
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" help:"maximum height for command output viewports in TUI" default:"8"`
}

type Logging struct {
	Log        bool   `yaml:"log" short:"l" help:"enable logging to file"`
	LogFile    string `yaml:"log_file" help:"log file path (enables logging)" default:"panix.log"`
	Debug      bool   `yaml:"debug" short:"d" help:"debug output (enables logging)"`
	CPUProfile string `yaml:"cpu_profile" help:"path for cpu profiling to file, declaring it enables it"`
}

func (f *Flags) SetDefault(reverse bool) {
	defaultHostname, _ := os.Hostname()
	toggle(reverse, &f.Config, "panix.yml", "")
	toggle(reverse, &f.OverrideLocalMachine, defaultHostname, "")
	toggle(reverse, &f.Timeout, 2*time.Hour, 0)
	toggle(reverse, &f.Tui.CommandOutputMaxHeight, 8, 0)
	toggle(reverse, &f.LogFile, "panix.log", "")
}

func (f *Flags) MergeConfWithCliFlags(cli Flags) error {
	f.SetDefault(true)
	if err := mergo.Merge(f, cli); err != nil {
		return err
	}
	f.SetDefault(false)
	return nil
}

func toggle[T comparable](reverse bool, p *T, def, zero T) {
	if reverse {
		if *p == def {
			*p = zero
		}
	} else {
		if *p == zero {
			*p = def
		}
	}
}
