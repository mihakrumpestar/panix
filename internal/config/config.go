package config

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/pkg/jsonerror"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
)

type Config struct {
	Flags       flags.Flags              `yaml:"flags" json:"flags" desc:"Flags (CLI and YAML)"`
	Fleet       *fleet.Fleet             `yaml:"fleet,required" json:"fleet" validate:"required" desc:"Fleet configuration"`
	ColorScheme *colorscheme.ColorScheme `yaml:"-" json:"-"`

	// Internal - exportable
	Snapshot Snapshot `yaml:"-" json:"snapshot"`

	// Internal - not exportable
	Phases []phase.Phase `yaml:"-" json:"phases"`
}

type Snapshot struct {
	PanixVersion string    `yaml:"-" json:"panix_version"`
	StartTime    time.Time `yaml:"-" json:"start_time"`

	Reason        SnaphsotReason       `yaml:"-" json:"reason"`
	SnapshotTime  time.Time            `yaml:"-" json:"snapshot_time"`
	WorkflowError *jsonerror.JSONError `yaml:"-" json:"workflow_error,omitempty"`
}

type SnaphsotReason string

const (
	SnaphsotReasonManual SnaphsotReason = "manual"
	SnaphsotReasonRetry  SnaphsotReason = "retry"
	SnaphsotReasonExit   SnaphsotReason = "exit"
)

func (sr SnaphsotReason) String() string {
	return string(sr)
}

func (c *Config) PostUnmarshalInit() {
	if c.ColorScheme == nil {
		c.ColorScheme = colorscheme.DefaultColorScheme()
	}

	if c.Fleet != nil {
		c.Fleet.RecalculateCachesOnly(c.Phases)
	}
}
