package logs_command

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/hayageek/threadsafe"
	"github.com/mihakrumpestar/panix/internal/pkg/safe_buffer"
	"github.com/mihakrumpestar/panix/internal/pkg/time_and_state"
)

type CommandLog struct {
	Description     string
	StatusIfRunning string
	StatusIfFailed  string
	Command         string
	stdInOutErr     *threadsafe.Slice[*safe_buffer.Buffer] // Each line is a separate buffer to allow line replacement
	TimeAndState    *time_and_state.TimeAndState
	Pty             *os.File
}

func NewCommandLog(description, statusIfRunning, statusIfFailed string, command, env []string) *CommandLog {
	commandLog := &CommandLog{
		Description:     description,
		StatusIfRunning: statusIfRunning,
		StatusIfFailed:  statusIfFailed,
		Command:         strings.Join(env, " ") + " " + strings.Join(command, " "),
		stdInOutErr:     threadsafe.NewSlice[*safe_buffer.Buffer](),
		TimeAndState:    time_and_state.NewTimeAndState(),
	}

	return commandLog
}

// String returns the string representation of all lines in StdInOutErr.
func (cl *CommandLog) String() string {
	return string(cl.Bytes())
}

// Bytes returns the byte representation of all lines in StdInOutErr.
func (cl *CommandLog) Bytes() []byte {
	values := cl.stdInOutErr.Values()
	if len(values) == 0 {
		return nil
	}

	var result bytes.Buffer
	result.Grow(estimateSize(values))

	for i, buf := range values {
		if i > 0 {
			result.WriteByte('\n')
		}
		result.Write(buf.Bytes())
	}

	return result.Bytes()
}

// estimateSize approximates the total size needed for all buffers.
func estimateSize(buffers []*safe_buffer.Buffer) int {
	total := 0
	for _, buf := range buffers {
		total += buf.Len() + 1 // +1 for potential newline
	}
	return total
}

// WriteString writes a string to the last line in StdInOutErr.
func (cl *CommandLog) WriteString(s string) (int, error) {
	return cl.Write([]byte(s))
}

// Write writes bytes to the last line in StdInOutErr.
func (cl *CommandLog) Write(p []byte) (int, error) {
	length := cl.stdInOutErr.Length()
	if length == 0 {
		cl.stdInOutErr.Append(safe_buffer.NewBuffer(nil))
		length = 1
	}

	stdInOutErr, ok := cl.stdInOutErr.Get(length - 1)
	if !ok {
		panic(fmt.Sprintf("command log %q: stdInOutErr index %d out of bounds (length=%d)", cl.Description, length-1, length))
	}

	return stdInOutErr.Write(p)
}

// WriteLineString writes a string as a new line to StdInOutErr.
func (cl *CommandLog) WriteLineString(s string) {
	cl.WriteLine([]byte(s))
}

// WriteLine writes bytes as a new line to StdInOutErr.
func (cl *CommandLog) WriteLine(p []byte) {
	cl.stdInOutErr.Append(safe_buffer.NewBuffer(p))
}

// ReplaceLastLine replaces the content of the last line in StdInOutErr.
func (cl *CommandLog) ReplaceLastLine(p []byte) {
	if cl.stdInOutErr.Length() == 0 {
		cl.WriteLine(p)
		return
	}

	index := cl.stdInOutErr.Length() - 1
	ok := cl.stdInOutErr.Set(index, safe_buffer.NewBuffer(p))
	if !ok {
		panic(fmt.Sprintf("command log %q: failed to replace line at index %d (length=%d)", cl.Description, index, cl.stdInOutErr.Length()))
	}
}
