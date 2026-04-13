package snapshot

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
)

func Capture(conf *config.Config, reason config.SnaphsotReason, workflowErr error) *config.Config {
	conf.Fleet.Recalculate(conf.Phases)

	confCopy := *conf

	confCopy.SnapshotTime = time.Now()
	confCopy.WorkflowError = workflowErr

	return &confCopy
}
