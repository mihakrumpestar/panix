package command

import (
	"strings"

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

	Xpath       xpath.Xpath `yaml:"-" json:"xpath,omitempty"`
	LabelXpath  xpath.Xpath `yaml:"-" json:"-"`
	OutputXpath xpath.Xpath `yaml:"-" json:"-"`
	ErrorXpath  xpath.Xpath `yaml:"-" json:"-"`
}

func NewCommandLog(phaseXpath xpath.Xpath, description, statusIfRunning, statusIfFailed string, command, env []string) *CommandLog {
	cmdXpath := phaseXpath.NewXpathWithAppend(description)

	commandLog := &CommandLog{
		Description:     description,
		StatusIfRunning: statusIfRunning,
		StatusIfFailed:  statusIfFailed,
		Command:         joinCommand(env, command),

		Output:       buffer.NewLinesBufVer(),
		TimeAndState: atomictimeandstate.New(),

		Xpath:       cmdXpath,
		LabelXpath:  cmdXpath.NewXpathWithAppend("label"),
		OutputXpath: cmdXpath.NewXpathWithAppend("output"),
		ErrorXpath:  cmdXpath.NewXpathWithAppend("error"),
	}

	return commandLog
}

// joinCommand joins env vars and command args into a shell-like LineBuf.
// Env vars are rendered as KEY='VALUE' (value quoted only if needed), command args
// are quoted as a whole if they contain spaces or special characters.
func joinCommand(env []string, cmd []string) *buffer.LineBuf {
	lineBuf := buffer.NewLineBuf()

	for i, envI := range env {
		if i > 0 {
			lineBuf.WriteByte(' ')
		}

		eqIdx := strings.IndexByte(envI, '=')
		if eqIdx < 0 {
			writeQuoted(lineBuf, envI)

			continue
		}

		lineBuf.WriteString(envI[:eqIdx+1]) // KEY=
		writeQuoted(lineBuf, envI[eqIdx+1:])
	}

	for i, arg := range cmd {
		if i > 0 || len(env) > 0 {
			lineBuf.WriteByte(' ')
		}

		writeQuoted(lineBuf, arg)
	}

	return lineBuf
}

// writeQuoted writes s to lb, wrapping in single quotes if it contains
// spaces, tabs, or quote characters.
func writeQuoted(lineBuf *buffer.LineBuf, str string) {
	if strings.ContainsAny(str, " \t'\"") {
		lineBuf.WriteByte('\'')
		lineBuf.WriteString(str)
		lineBuf.WriteByte('\'')
	} else {
		lineBuf.WriteString(str)
	}
}
