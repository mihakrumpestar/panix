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
	CommandOutputMaxHeight int  `yaml:"command_output_max_height" flag:"commandOutputMaxHeight" env:"COMMAND_OUTPUT_MAX_HEIGHT" desc:"maximum height for command output viewports in TUI"`
}

type Logging struct {
	Verbose bool `yaml:"verbose" flag:"verbose v" desc:"verbose output"`
	// Developers only
	Debug      bool   `yaml:"debug" flag:"debug d" desc:"debug output"`
	CPUProfile string `yaml:"cpu_profile" flag:"cpuprofile" desc:"path for cpu profiling to file, declaring it enables it"`
}

func (f *Flags) SetDefault(reverse bool) {
	defaultHostname, _ := os.Hostname()

	setDefault(reverse, &f.Config, "panix.yml", "")
	setDefault(reverse, &f.OverrideLocalMachine, defaultHostname, "")
	setDefault(reverse, &f.Timeout, 2*time.Hour, time.Duration(0))
	setDefault(reverse, &f.Tui.CommandOutputMaxHeight, 8, 0)
}

func (f *Flags) MergeConfWithCliFlags(cli Flags) error {

	f.SetDefault(true)

	err := mergo.Merge(f, cli)
	if err != nil {
		return err
	}

	f.SetDefault(false)

	return nil
}

// Helpers

func setDefault[T comparable](reverse bool, current *T, def, zero T) {
	if reverse {
		clearDefault(current, def, zero)
	} else {
		applyDefault(current, def, zero)
	}
}

func applyDefault[T comparable](current *T, def, zero T) {
	if *current == zero {
		*current = def
	}
}

func clearDefault[T comparable](current *T, def, zero T) {
	if *current == def {
		*current = zero
	}
}
