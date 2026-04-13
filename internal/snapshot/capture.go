package snapshot

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/pkg/errorjson"
)

func Capture(conf *config.Config, reason config.SnaphsotReason, workflowErr error) *config.Config {
	conf.Fleet.Recalculate(conf.Phases)

	confCopy := *conf

	confCopy.SnapshotTime = time.Now()
	confCopy.SnapshotReason = reason
	confCopy.WorkflowError = errorjson.New(workflowErr)

	return &confCopy
}
