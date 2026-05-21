package tui

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var errMachinesFailed = errors.New("one or more machines failed")

func NewHeadless(ctx context.Context, conf *config.Config) error {
	workflowI, err := workflow.NewWorkflow(ctx, conf)
	if err != nil {
		return errors.Wrap(err, "failed to create workflow")
	}

	log.Info().
		Str("event", "workflow_start").
		Int("machine_count", workflowI.MachineCount()).
		Msg("workflow started")

	err = workflowI.StartWorkflow()

	logFinalState(conf)

	if err != nil {
		return errors.Wrap(err, "workflow execution failed")
	}

	return nil
}

// Duration that outputs as float64 in seconds.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).Seconds()) //nolint:wrapcheck
}

type MachineState struct {
	Xpath    xpath.Xpath `json:"xpath"`
	Status   string      `json:"status"`
	Phase    string      `json:"phase"`
	Duration Duration    `json:"duration"`
	Error    string      `json:"error,omitempty"`
}

func logFinalState(conf *config.Config) {
	conf.Fleet.Recalculate(conf.Phases) // Needed, since this is generally only done in TUI

	anyFailed := false
	states := make([]MachineState, 0)

	for _, fleetLeaf := range conf.Fleet.AllMachines() {
		machineState := fleetLeaf.Machine.State.Load()

		entry := MachineState{
			Xpath:    fleetLeaf.Machine.Xpath,
			Status:   string(machineState.Status),
			Phase:    string(machineState.Phase),
			Duration: Duration(fleetLeaf.Machine.Logs.DurationAndErrorCache.Duration),
		}

		if machineState.Status == "failed" {
			anyFailed = true

			if machineState.Error != nil {
				entry.Error = machineState.Error.Error()
			}
		}

		states = append(states, entry)
	}

	var workflowErr error
	if anyFailed {
		workflowErr = errMachinesFailed
	}

	sublog := log.With().Str("event", "workflow_end").Logger()

	logger.ResultEvent(sublog, "workflow completed", workflowErr, func(event *zerolog.Event) {
		event.Interface("machines", states)
	})
}
