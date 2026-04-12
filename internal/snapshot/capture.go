package snapshot

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func Capture(conf *config.Config, reason config.SnaphsotReason, workflowErr error, workflowPhases []phases.Phase) *config.Config {
	conf.Fleet.Recalculate(workflowPhases)

	var confCopy *config.Config
	*confCopy = *conf

	confCopy.SnapshotTime = time.Now()
	confCopy.WorkflowError = workflowErr

	return confCopy
}
