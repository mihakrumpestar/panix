package config_flags

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type Flags struct {
	Tags                 []string       `yaml:"tags"`
	Bootstrap            Bootstrap      `yaml:"bootstrap"`
	RequireAllSuccess    bool           `yaml:"requireAllSuccess"`
	OverrideLocalMachine string         `yaml:"overrideLocalMachine"`
	DryRun               bool           `yaml:"dryRun"`
	DryRunWithStatus     bool           `yaml:"dryRunWithStatus"`
	Timeout              time.Duration  `yaml:"timeout"`
	SkipPhases           []phases.Phase `yaml:"skipPhases"`

	Logging Logging `yaml:",squash"`
}

type Bootstrap struct {
	Only         bool `yaml:"only"`
	DisableAuto  bool `yaml:"disableAuto"`
	DisableDisko bool `yaml:"disableDisko"`
}

type Logging struct {
	Verbose bool `yaml:"verbose"`
	// Developers only
	Debug      bool   `yaml:"debug"`
	Cpuprofile string `yaml:"cpuprofile"`
}

func (f *Flags) Setup() {
	// Convert timeout from miliseconds to seconds for duration
	f.Timeout *= time.Second

	if f.Logging.Debug {
		f.Logging.Verbose = true
	}
}
