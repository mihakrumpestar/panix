package config

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type Config struct {
	Flags       *flags.Flags `yaml:"flags" json:"flags"`
	Fleet       *fleet.Fleet `yaml:"fleet,required" json:"fleet" validate:"required"`
	ColorScheme *ColorScheme `yaml:"-" json:"-" validate:"-"`

	// Internal - exportable
	PanixVersion string    `yaml:"-" json:"panix_version" validate:"-"`
	StartTime    time.Time `yaml:"-" json:"start_time" validate:"-"`
	SnapshotTime time.Time `yaml:"-" json:"snapshot_time" validate:"-"`

	// Internal - not exportable
	Phases []phases.Phase `yaml:"-" json:"-" validate:"-"`
}
