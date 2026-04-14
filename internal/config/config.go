package config

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type Config struct {
	Flags       *flags.Flags             `yaml:"flags" json:"flags"`
	Fleet       *fleet.Fleet             `yaml:"fleet,required" json:"fleet" validate:"required"`
	ColorScheme *colorscheme.ColorScheme `yaml:"-" json:"-" validate:"-"`

	// Internal - exportable
	PanixVersion string    `yaml:"-" json:"panix_version" validate:"-"`
	StartTime    time.Time `yaml:"-" json:"start_time" validate:"-"`

	// // Filled on snapshot

	SnapshotTime   time.Time            `yaml:"-" json:"snapshot_time" validate:"-"`
	SnapshotReason SnaphsotReason       `yaml:"-" json:"reason"`
	WorkflowError  *errorjson.ErrorJSON `yaml:"-" json:"workflow_error,omitempty"`

	// Internal - not exportable
	Phases []phases.Phase `yaml:"-" json:"phases" validate:"-"`
}

type SnaphsotReason string

const (
	SnaphsotReasonManual SnaphsotReason = "manual"
	SnaphsotReasonRetry  SnaphsotReason = "retry"
	SnaphsotReasonExit   SnaphsotReason = "exit"
)

func (c *Config) PostUnmarshalInit() {
	if c.Flags == nil {
		c.Flags = &flags.Flags{}
		c.Flags.DefautlIfNoTTY()
	}

	if c.ColorScheme == nil {
		c.ColorScheme = colorscheme.DefaultColorScheme()
	}

	if len(c.Phases) == 0 {
		c.Phases = phases.DeployPhasesInOrder()
	}

	if c.Fleet != nil {
		c.Fleet.PostUnmarshalInit()
		c.Fleet.RecalculateCachesOnly(c.Phases)
	}
}
