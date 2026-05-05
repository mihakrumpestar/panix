package snapshot

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/pkg/jsonerror"
)

func Capture(conf *config.Config, reason config.SnaphsotReason, workflowErr error) *config.Config {
	conf.Fleet.Recalculate(conf.Phases)

	confCopy := *conf

	confCopy.Snapshot.SnapshotTime = time.Now()
	confCopy.Snapshot.Reason = reason
	confCopy.Snapshot.WorkflowError = jsonerror.New(workflowErr)

	return &confCopy
}
