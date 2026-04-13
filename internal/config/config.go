package config

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/colorscheme"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/tui/phasestatus"
	"github.com/mihakrumpestar/panix/internal/tui/statstable"
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
	SnapshotReason SnaphsotReason       `json:"reason"`
	WorkflowError  *errorjson.ErrorJSON `json:"workflow_error,omitempty"`

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

	if c.Fleet == nil {
		return
	}

	c.initFleetInternalFields()
	c.Fleet.RecalculateCachesOnly(c.Phases)
}

func (c *Config) initFleetInternalFields() {
	fl := c.Fleet

	if fl.Logs == nil {
		fl.Logs = logs.New(&fl.Attributes)
	} else {
		initLogs(fl.Logs, &fl.Attributes)
	}

	if fl.StatsTable == nil {
		fl.StatsTable = statstable.NewStatsTable()
	}

	if fl.PhaseStatus == nil {
		fl.PhaseStatus = phasestatus.NewPhaseStatus()
	}

	for _, flakePair := range fl.Flakes.Pairs() {
		fV := flakePair.Value
		initFlakeLogs(flakePair.Key, fl, fV)

		for _, cfgPair := range fV.Configurations.Pairs() {
			cV := cfgPair.Value
			initConfigurationLogs(cfgPair.Key, fV, cV)

			for _, mPair := range cV.Machines.Pairs() {
				mV := mPair.Value
				if mV == nil {
					continue
				}

				initMachineLogs(mPair.Key, cV, mV)

				if mV.State == nil {
					mV.State = &machine.State{ActiveSSH: machine.SSHTypeRegular}
				}
			}
		}
	}
}

func initFlakeLogs(name string, parent *fleet.Fleet, f *flake.Flake) {
	if f.Logs == nil {
		f.Logs = logs.New(&f.Attributes)
	} else {
		initLogs(f.Logs, &f.Attributes)
	}

	ensureAttributes(&f.Attributes, name, &parent.Attributes)
}

func initConfigurationLogs(name string, parent *flake.Flake, c *configuration.Configuration) {
	if c.Logs == nil {
		c.Logs = logs.New(&c.Attributes)
	} else {
		initLogs(c.Logs, &c.Attributes)
	}

	ensureAttributes(&c.Attributes, name, &parent.Attributes)
}

func initMachineLogs(name string, parent *configuration.Configuration, m *machine.Machine) {
	if m.Logs == nil {
		m.Logs = logs.New(&m.Attributes)
	} else {
		initLogs(m.Logs, &m.Attributes)
	}

	ensureAttributes(&m.Attributes, name, &parent.Attributes)
}

func initLogs(logInst *logs.Logs, attr *attributes.Attributes) {
	if logInst == nil {
		return
	}
	logInst.SetAttributes(attr)
	if logInst.PhaseLogs == nil {
		logInst.PhaseLogs = phase.NewPhaseLogs()
	}
}

func ensureAttributes(attr *attributes.Attributes, name string, parentAttr *attributes.Attributes) {
	if attr.Name == "" {
		attr.Name = name
	}
	if attr.Xpath.String() == "" && parentAttr != nil {
		attr.Xpath = parentAttr.Xpath.NewXpathWithAppend(name)
	}
}
