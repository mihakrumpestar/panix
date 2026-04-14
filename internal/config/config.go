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
	Snapshot Snapshot `yaml:"-" json:"snapshot"`

	// Internal - not exportable
	Phases []phases.Phase `yaml:"-" json:"phases" validate:"-"`
}

type Snapshot struct {
	PanixVersion string    `yaml:"-" json:"panix_version" validate:"-"`
	StartTime    time.Time `yaml:"-" json:"start_time" validate:"-"`

	Reason        SnaphsotReason       `yaml:"-" json:"reason"`
	SnapshotTime  time.Time            `yaml:"-" json:"snapshot_time" validate:"-"`
	WorkflowError *errorjson.ErrorJSON `yaml:"-" json:"workflow_error,omitempty"`
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
	if c.Flags == nil {
		c.Flags = &flags.Flags{}
		c.Flags.DefautlIfNoTTY()
	}

	if c.ColorScheme == nil {
		c.ColorScheme = colorscheme.DefaultColorScheme()
	}

	if c.Fleet != nil {
		c.Fleet.RecalculateCachesOnly(c.Phases)
	}
}
