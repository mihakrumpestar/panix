package config_flags

import (
	"os"
	"time"

	"dario.cat/mergo"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type Flags struct {
	Config               string         `yaml:"config" flag:"config c" desc:"config file"` // CLI-only
	Tags                 []string       `yaml:"tags" flag:"tags t" desc:"filter machines by tags (flakes, configurations and machine names are already registered as tags, children inherit all parent tags)"`
	Bootstrap            Bootstrap      `yaml:"bootstrap" flag:"bootstrap b"`
	RequireAllSuccess    bool           `yaml:"require_all_success" flag:"requireAllSuccess" env:"REQUIRE_ALL_SUCCESS" desc:"abort & rollback if any task fails, primarily for CI/CD"`
	OverrideLocalMachine string         `yaml:"override_local_machine" flag:"overrideLocalMachine" env:"OVERRIDE_LOCAL_MACHINE" desc:"hostname of the machine that is local (won't use ssh to connect to it)"`
	DryRun               bool           `yaml:"dry_run" flag:"dryRun d" env:"DRY_RUN" desc:"show what would be done without executing"`
	DryRunWithStatus     bool           `yaml:"dry_run_with_status" flag:"dryRunWithStatus" env:"DRY_RUN_WITH_STATUS" desc:"show what would be done without executing, but with real status query"`
	Timeout              time.Duration  `yaml:"timeout" flag:"timeout" desc:"timeout for TUI"`
	SkipPhases           []phases.Phase `yaml:"skip_phases" flag:"skipPhases s" env:"SKIP_PHASES" desc:"declare phases to skip"`
	ExitOnComplete       bool           `yaml:"exit_on_complete" flag:"exitOnComplete" env:"EXIT_ON_COMPLETE" desc:"exit TUI immediately when workflow completes (otherwise stays open until user quits)"`

	Tui     `yaml:"tui" flag:"tui"`
	Logging `yaml:"logging" flag:"logging"`
}

type Bootstrap struct {
	Only         bool `yaml:"only" flag:"only" desc:"only initializes uninitialized machines"`
	DisableAuto  bool `yaml:"disable_auto" flag:"disableAuto" env:"DISABLE_AUTO" desc:"disable automatic bootstrap (even if target machine does not have NixOS installed)"`
	DisableDisko bool `yaml:"disable_disko" flag:"disableDisko" env:"DISABLE_DISKO" desc:"disables building, transfer and bootstrap of disko tool"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"show_all_build_logs" flag:"showAllBuildLogs" env:"SHOW_BUILD_LOGS" desc:"show all build logs in TUI"`
	ShowActiveOnly         bool `yaml:"show_active_only" flag:"showActiveOnly" env:"SHOW_ACTIVE_ONLY" desc:"show only running or errored logs in TUI build logs"`
	ShowCommandsInLabels   bool `yaml:"show_commands_in_labels" flag:"showCommandsInLabels" env:"SHOW_COMMANDS_IN_LABELS" desc:"show commands instead of descriptions as labels in build logs"`
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" flag:"commandOutputMaxHeight" env:"COMMAND_OUTPUT_MAX_HEIGHT" desc:"maximum height for command output viewports in TUI"`
}

type Logging struct {
	Log        bool   `yaml:"log" flag:"log l" desc:"enable logging to file"`
	LogFile    string `yaml:"log_file" flag:"logFile" desc:"log file path (enables logging)" validate:"omitempty,filepath"`
	Debug      bool   `yaml:"debug" flag:"debug d" desc:"debug output (enables logging)"`
	CPUProfile string `yaml:"cpu_profile" flag:"cpuProfile" desc:"path for cpu profiling to file, declaring it enables it" validate:"omitempty,filepath"`
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
