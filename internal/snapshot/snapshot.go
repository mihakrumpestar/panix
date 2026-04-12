package snapshot

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

type Reason string

const (
	ReasonManual Reason = "manual"
	ReasonRetry  Reason = "retry"
	ReasonExit   Reason = "exit"
)

type Snapshot struct {
	Version       string         `json:"version"`
	AppStartTime  int64          `json:"app_start_time"`
	SnapshotTime  int64          `json:"snapshot_time"`
	Reason        Reason         `json:"reason"`
	Phases        []phases.Phase `json:"phases"`
	Flags         flags.Flags    `json:"flags"`
	Fleet         *config.Fleet  `json:"fleet"`
	WorkflowError string         `json:"workflow_error,omitempty"`
}

func epoch(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}

	return t.Unix()
}
