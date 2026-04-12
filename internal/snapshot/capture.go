package snapshot

import (
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/workflow/phases"
)

func Capture(conf *config.Config, reason Reason, workflowErr error) (*Snapshot, error) {
	conf.Fleet.CalculateDurationAndError(conf.Phases)

	var workflowErrStr string
	if workflowErr != nil {
		workflowErrStr = workflowErr.Error()
	}

	workflowPhases := conf.Phases
	if workflowPhases == nil {
		workflowPhases = []phases.Phase{}
	}

	return &Snapshot{
		Version:       conf.PanixVersion,
		AppStartTime:  epoch(conf.StartTime),
		SnapshotTime:  epoch(time.Now()),
		Reason:        reason,
		Phases:        workflowPhases,
		Flags:         *conf.Flags,
		Fleet:         conf.Fleet,
		WorkflowError: workflowErrStr,
	}, nil
}
