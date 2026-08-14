package config

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/jsonerror"
	"github.com/mihakrumpestar/panix/pkg/nixver"
)

// Config is the main internal and external state source
//
//nolint:lll
type Config struct {
	Nix *nixver.Info `yaml:"-" json:"nix,omitempty"`

	// Internal, exportable
	Snapshot Snapshot      `yaml:"-" json:"snapshot"`
	Phases   []phase.Phase `yaml:"-" json:"phases"`

	Flags       flags.Flags                   `yaml:"flags" json:"flags" desc:"Flags (CLI and YAML)"`
	OutputTypes installable.CustomOutputTypes `yaml:"output_types" json:"output_types,omitempty" desc:"Custom output type presets, applied as defaults to installables of that type"`
	Fleet       *fleet.Fleet                  `yaml:"fleet,required" json:"fleet" validate:"required" desc:"Fleet configuration"`
	ColorScheme *colorscheme.ColorScheme      `yaml:"-" json:"-"`
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
		c.Fleet.PostUnmarshalInit()
		c.Fleet.RecalculateCachesOnly(c.Phases)
	}
}
