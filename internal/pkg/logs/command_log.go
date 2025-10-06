package logs

import (
	"bytes"
	"os"

	"github.com/hayageek/threadsafe"
	"github.com/mihakrumpestar/panix/internal/pkg/safe_buffer"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
	"go.uber.org/atomic"
)

type CommandLog struct {
	Command      atomic.String
	stdInOutErr  *threadsafe.Slice[*safe_buffer.Buffer] // Each line is a separate buffer to allow line replacement
	TimeAndState *time_and_state.TimeAndState
	Pty          *os.File
}

func NewCommandLog() *CommandLog {
	return &CommandLog{
		stdInOutErr:  threadsafe.NewSlice[*safe_buffer.Buffer](),
		TimeAndState: time_and_state.NewTimeAndState(),
	}
}

// Bytes wrapper
func (cl *CommandLog) String() string {
	return string(cl.Bytes())
}

// Bytes returns the byte representation of all lines in StdInOutErr
func (cl *CommandLog) Bytes() []byte {
	var result bytes.Buffer
	for i, buf := range cl.stdInOutErr.Values() {
		if i > 0 {
			result.WriteByte('\n')
		}
		result.Write(buf.Bytes())
	}
	return result.Bytes()
}

// Write wrapper
func (cl *CommandLog) WriteString(s string) (int, error) {
	return cl.Write([]byte(s))
}

// Write writes bytes to the last line in StdInOutErr
func (cl *CommandLog) Write(p []byte) (int, error) {
	// If there are no lines, create the first one
	if cl.stdInOutErr.Length() == 0 {
		cl.stdInOutErr.Append(safe_buffer.NewBuffer(nil))
	}

	stdInOutErr, ok := cl.stdInOutErr.Get(cl.stdInOutErr.Length() - 1)
	if !ok {
		panic("stdInOutErr does not have element on specified index")
	}

	return stdInOutErr.Write(p)
}

// WriteLine wrapper
func (cl *CommandLog) WriteLineString(s string) {
	cl.WriteLine([]byte(s))
}

// WriteLine writes a new line to StdInOutErr
func (cl *CommandLog) WriteLine(p []byte) {
	cl.stdInOutErr.Append(safe_buffer.NewBuffer(p))
}

// ReplaceLastLine replaces the content of the last line in StdInOutErr
func (cl *CommandLog) ReplaceLastLine(p []byte) {
	if cl.stdInOutErr.Length() == 0 {
		cl.WriteLine(p)
	} else {
		ok := cl.stdInOutErr.Set(cl.stdInOutErr.Length()-1, safe_buffer.NewBuffer(p))
		if !ok {
			panic("stdInOutErr does not have element on specified index")
		}
	}
}
