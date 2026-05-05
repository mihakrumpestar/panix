package command

import (
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
)

type CommandLog struct {
	// On creation
	Description     string `yaml:"-" json:"description,omitempty"`
	StatusIfRunning string `yaml:"-" json:"-"`
	StatusIfFailed  string `yaml:"-" json:"-"`
	Command         string `yaml:"-" json:"command,omitempty"`

	// Mutate
	Output       *AtomicCommandOutput
	TimeAndState *atomictimeandstate.AtomicTimeAndState `yaml:"-" json:"time_and_state,omitempty"`
}

func NewCommandLog(description, statusIfRunning, statusIfFailed string, command, env []string) *CommandLog {
	commandLog := &CommandLog{
		Description:     description,
		StatusIfRunning: statusIfRunning,
		StatusIfFailed:  statusIfFailed,
		Command:         joinCommand(slices.Concat(env, command)),

		Output:       NewAtomicCommandOutput(),
		TimeAndState: atomictimeandstate.New(),
	}

	return commandLog
}

// joinCommand joins command parts into a shell-like string, quoting parts that contain spaces.
func joinCommand(parts []string) string {
	quoted := make([]string, len(parts))

	for i, part := range parts {
		if strings.Contains(part, " ") {
			quoted[i] = "'" + part + "'"
		} else {
			quoted[i] = part
		}
	}

	return strings.Join(quoted, " ")
}
