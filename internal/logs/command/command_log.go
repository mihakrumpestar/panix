package command

import (
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomictimeandstate"
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
		Command:         strings.Join(slices.Concat(env, command), " "),
		Output:          NewAtomicCommandOutput(),
		TimeAndState:    atomictimeandstate.New(),
	}

	return commandLog
}
