package config_flags

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

// Flags holds all configuration flags
// yaml tags are used by koanf for config file and env vars
// desc tags are used by sflags for CLI help text
// flag tags can customize the CLI flag name:
//   - "name" - uses 'name' as the flag name (with parent prefix)
//   - "~name" - uses 'name' without parent prefix
//   - "name n" - adds short alias 'n'
type Flags struct {
	Config               string         `yaml:"config c" desc:"config file"` // CLI-only
	Tags                 []string       `yaml:"tags t" desc:"filter machines by tags (flakes, configurations and machine names are already registered as tags, children inherit all parent tags)"`
	Bootstrap            Bootstrap      `yaml:"bootstrap b"`
	RequireAllSuccess    bool           `yaml:"requireAllSuccess" env:"REQUIRE_ALL_SUCCESS" desc:"abort & rollback if any task fails, primarily for CI/CD"`
	OverrideLocalMachine string         `yaml:"overrideLocalMachine" env:"OVERRIDE_LOCAL_MACHINE" desc:"hostname of the machine that is local (won't use ssh to connect to it)"`
	DryRun               bool           `yaml:"dryRun d" env:"DRY_RUN" desc:"show what would be done without executing"`
	DryRunWithStatus     bool           `yaml:"dryRunWithStatus ds" env:"DRY_RUN_WITH_STATUS" desc:"show what would be done without executing, but with real status query"`
	Timeout              time.Duration  `yaml:"timeout t" desc:"timeout for TUI in seconds"`
	SkipPhases           []phases.Phase `yaml:"skipPhases sp" env:"SKIP_PHASES" desc:"declare phases to skip"`

	Tui     `yaml:"tui"`
	Logging `yaml:"logging,squash"`
}

type Bootstrap struct {
	Only         bool `yaml:"only" desc:"only initializes uninitialized machines"`
	DisableAuto  bool `yaml:"disableAuto" env:"DISABLE_AUTO" desc:"disable automatic bootstrap (even if target machine does not have NixOS installed)"`
	DisableDisko bool `yaml:"disableDisko" env:"DISABLE_DISKO" desc:"disables building, transfer and bootstrap of disko tool"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"showAllBuildLogs" env:"SHOW_BUILD_LOGS" desc:"show all build logs in TUI"`
	CommandOutputMaxHeight int  `yaml:"commandOutputMaxHeight" env:"COMMAND_OUTPUT_MAX_HEIGHT" desc:"maximum height for command output viewports in TUI"`
}

type Logging struct {
	Verbose bool `yaml:"verbose v" desc:"verbose output"`
	// Developers only
	Debug      bool   `yaml:"debug d" desc:"debug output"`
	Cpuprofile string `yaml:"cpuprofile" desc:"path for cpu profiling to file, declaring it enables it"`
}

func (f *Flags) Setup() {
	// Convert timeout from seconds (int value) to duration
	// The raw value comes from config as seconds, convert to Duration
	f.Timeout *= time.Second
}
