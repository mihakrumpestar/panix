package command

import (
	"github.com/mihakrumpestar/panix/pkg/atomic/atomictimeandstate"
	"github.com/mihakrumpestar/panix/pkg/buffer"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

type CommandLog struct {
	Description     string          `yaml:"-" json:"description,omitempty"`
	StatusIfRunning string          `yaml:"-" json:"-"`
	StatusIfFailed  string          `yaml:"-" json:"-"`
	Command         *buffer.LineBuf `yaml:"-" json:"command,omitempty"`

	Output         *buffer.LinesBufVer                    `yaml:"-" json:"output"`
	TimeAndState   *atomictimeandstate.AtomicTimeAndState `yaml:"-" json:"time_and_state,omitempty"`
	PendingNewline bool                                   `yaml:"-" json:"-"`
	CarriageReturn bool                                   `yaml:"-" json:"-"` // cursor at column 0 after trailing \r

	Xpath       xpath.Xpath `yaml:"-" json:"xpath,omitzero"`
	LabelXpath  xpath.Xpath `yaml:"-" json:"-"`
	OutputXpath xpath.Xpath `yaml:"-" json:"-"`
	ErrorXpath  xpath.Xpath `yaml:"-" json:"-"`
}

func NewCommandLog(phaseXpath xpath.Xpath, description, statusIfRunning, statusIfFailed string, command []string) *CommandLog {
	cmdXpath := phaseXpath.NewXpathWithAppend(description)

	commandLog := &CommandLog{
		Description:     description,
		StatusIfRunning: statusIfRunning,
		StatusIfFailed:  statusIfFailed,
		Command:         joinCommand(command),

		Output:       buffer.NewLinesBufVer(),
		TimeAndState: atomictimeandstate.New(),

		Xpath: cmdXpath,
	}

	commandLog.initDerivedXpaths()

	return commandLog
}

// PostUnmarshalInit recomputes the derived xpaths that json:"-" omits; call it
// after JSON deserialization.
func (cl *CommandLog) PostUnmarshalInit() {
	cl.initDerivedXpaths()
}

func (cl *CommandLog) initDerivedXpaths() {
	cl.LabelXpath = cl.Xpath.NewXpathWithAppend("label")
	cl.OutputXpath = cl.Xpath.NewXpathWithAppend("output")
	cl.ErrorXpath = cl.Xpath.NewXpathWithAppend("error")
}

// joinCommand renders argv as a space-joined line. The SSH transport
// pre-quotes its elements (pkg/shellquote), so the log adds no quotes of its
// own: re-quoting would double them and render ssh lines unparseable.
func joinCommand(cmd []string) *buffer.LineBuf {
	lineBuf := buffer.NewLineBuf()

	for i, arg := range cmd {
		if i > 0 {
			lineBuf.WriteByte(' ')
		}

		lineBuf.WriteString(arg)
	}

	return lineBuf
}
