package command

import (
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/pkg/linesbuffer"
)

type CommandLog struct {
	Description     string `yaml:"-" json:"description,omitempty"`
	StatusIfRunning string `yaml:"-" json:"-"`
	StatusIfFailed  string `yaml:"-" json:"-"`
	Command         string `yaml:"-" json:"command,omitempty"`

	Output            *linesbuffer.LinesBuffer               `yaml:"-" json:"output"`
	TimeAndState      *atomictimeandstate.AtomicTimeAndState `yaml:"-" json:"time_and_state,omitempty"`
	PendingNewline    bool                                   `yaml:"-" json:"-"`
	CarriageReturn    bool                                   `yaml:"-" json:"-"` // cursor at column 0 after trailing \r
}

func NewCommandLog(description, statusIfRunning, statusIfFailed string, command, env []string) *CommandLog {
	commandLog := &CommandLog{
		Description:     description,
		StatusIfRunning: statusIfRunning,
		StatusIfFailed:  statusIfFailed,
		Command:         joinCommand(slices.Concat(env, command)),

		Output:       linesbuffer.NewAtomic(),
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
