package tui

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/logger"
	"github.com/mihakrumpestar/panix/internal/workflow"
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

	err = workflowI.StartWorkflow() //nolint:contextcheck // False positive lint

	logFinalState(workflowI)

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

type machineState struct {
	Status   string   `json:"status"`
	Phase    string   `json:"phase"`
	Duration Duration `json:"duration"`
	Error    string   `json:"error,omitempty"`
}

func logFinalState(workflow *workflow.Workflow) {
	anyFailed := false
	states := make(map[string]machineState)

	targetsLogs := workflow.State().TargetsLogs
	workflowPhases := workflow.WorkflowPhases()

	workflow.RootTree(func(_ int, machine *config.Machine) {
		machineStateI := targetsLogs.MustGet(machine.Xpath).ComputeMachineState(workflowPhases)
		if machineStateI.Status == "" {
			return
		}

		entry := machineState{
			Status:   string(machineStateI.Status),
			Phase:    string(machineStateI.Phase),
			Duration: Duration(machineStateI.Duration),
		}

		if machineStateI.Status == "failed" {
			anyFailed = true
			entry.Error = machineStateI.Error.Error()
		}

		states[machine.Xpath.String()] = entry
	})

	var workflowErr error
	if anyFailed {
		workflowErr = errMachinesFailed
	}

	sublog := log.With().Str("event", "workflow_end").Logger()
	logger.ResultEvent(sublog, "workflow completed", workflowErr, func(event *zerolog.Event) {
		event.Interface("machines", states)
	})
}
