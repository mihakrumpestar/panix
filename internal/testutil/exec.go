package testutil

import (
	"context"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/logs/phaselogs"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// NewDryRunExecutioner builds a dry-run executioner bound to the given machine
// and deploy phase. Command argv is recorded in the phase log before the early
// return, so tests can assert exact command lines without executing anything.
func NewDryRunExecutioner(
	t *testing.T,
	mach *machine.Machine,
	deployPhase phase.Phase,
) (*executioner.Executioner, *phaselogs.PhaseLog) {
	t.Helper()

	phaseLog := phaselogs.NewPhaseLog()
	exc := executioner.NewExecutioner(executioner.ExecutionerConf{
		Ctx:          context.Background(),
		DryRun:       true,
		Xpath:        xpath.New("test"),
		Machine:      mach,
		Phase:        deployPhase,
		PhaseLog:     phaseLog,
		OnUpdateHook: func() {},
	})

	return exc, phaseLog
}

// CommandLines returns every recorded command line in execution order,
// rendered as the verbatim space-joined argv (joinCommand never re-quotes).
func CommandLines(t *testing.T, phaseLog *phaselogs.PhaseLog) []string {
	t.Helper()

	var out []string

	for i := 0; ; i++ {
		log, ok := phaseLog.CommandLogs.Get(i)
		if !ok {
			break
		}

		out = append(out, commandLineOf(log))
	}

	return out
}

// LastCommandLine fails the test when no command was recorded.
func LastCommandLine(t *testing.T, phaseLog *phaselogs.PhaseLog) string {
	t.Helper()

	last, ok := phaseLog.CommandLogs.Last()
	if !ok {
		t.Fatal("no command was recorded")
	}

	return commandLineOf(last)
}

func commandLineOf(log *command.CommandLog) string {
	if log.Command == nil {
		return ""
	}

	return log.Command.String()
}
