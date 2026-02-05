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
	Config               string         `yaml:"config" desc:"config file"` // CLI-only
	Tags                 []string       `yaml:"tags" desc:"filter machines by tags (flakes, configurations and machine names are already registered as tags, children inherit all parent tags)"`
	Bootstrap            Bootstrap      `yaml:"bootstrap"`
	RequireAllSuccess    bool           `yaml:"requireAllSuccess" desc:"abort & rollback if any task fails, primarily for CI/CD"`
	OverrideLocalMachine string         `yaml:"overrideLocalMachine" desc:"hostname of the machine that is local (won't use ssh to connect to it)"`
	DryRun               bool           `yaml:"dryRun" desc:"show what would be done without executing"`
	DryRunWithStatus     bool           `yaml:"dryRunWithStatus" desc:"show what would be done without executing, but with real status query"`
	Timeout              time.Duration  `yaml:"timeout" desc:"timeout for TUI in seconds"`
	SkipPhases           []phases.Phase `yaml:"skipPhases" desc:"declare phases to skip"`

	Tui     Tui     `yaml:"tui"`
	Logging Logging `yaml:",squash"`
}

type Bootstrap struct {
	Only         bool `yaml:"only" desc:"only initializes uninitialized machines"`
	DisableAuto  bool `yaml:"disableAuto" desc:"disable automatic bootstrap (even if target machine does not have NixOS installed)"`
	DisableDisko bool `yaml:"disableDisko" desc:"disables building, transfer and bootstrap of disko tool"`
}

type Tui struct {
	ShowAllBuildLogs       bool `yaml:"showAllBuildLogs" desc:"show all build logs in TUI"`
	CommandOutputMaxHeight int  `yaml:"commandOutputMaxHeight" desc:"maximum height for command output viewports in TUI"`
}

type Logging struct {
	Verbose bool `yaml:"verbose" desc:"verbose output"`
	// Developers only
	Debug      bool   `yaml:"debug" desc:"debug output"`
	Cpuprofile string `yaml:"cpuprofile" desc:"path for cpu profiling to file, declaring it enables it"`
}

func (f *Flags) Setup() {
	// Convert timeout from seconds (int value) to duration
	// The raw value comes from config as seconds, convert to Duration
	f.Timeout *= time.Second
}
