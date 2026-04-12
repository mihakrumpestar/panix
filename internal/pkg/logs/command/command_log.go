package command

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/pkg/atomicslice"
	"github.com/mihakrumpestar/panix/internal/pkg/safebuffer"
	"github.com/mihakrumpestar/panix/internal/pkg/timeandstate"
	"github.com/pkg/errors"
)

var nixBuildOutputPrefix = []byte(`[{"drvPath":"/nix/store/`)

type CommandLog struct {
	// On creation
	Description     string `json:"description"`
	StatusIfRunning string `json:"-"`
	StatusIfFailed  string `json:"-"`
	Command         string `json:"command"`

	// Mutate
	Output       *atomicslice.AtomicSlice[*safebuffer.Buffer] `json:"output,omitempty"` // Std In and Out; Each line is a separate buffer to allow line replacement
	TimeAndState *timeandstate.AtomicTimeAndState             `json:"time_and_state"`
}

func NewCommandLog(description, statusIfRunning, statusIfFailed string, command, env []string) *CommandLog {
	commandLog := &CommandLog{
		Description:     description,
		StatusIfRunning: statusIfRunning,
		StatusIfFailed:  statusIfFailed,
		Command:         strings.Join(slices.Concat(env, command), " "),
		Output:          atomicslice.New[*safebuffer.Buffer](),
		TimeAndState:    timeandstate.New(),
	}

	return commandLog
}

// String returns the string representation of all lines in StdInOutErr.
func (cl *CommandLog) String() string {
	return string(cl.Bytes())
}

func (cl *CommandLog) StringForBuildLogs() string {
	return string(cl.BytesForBuildLogs())
}

// Bytes returns the byte representation of all lines in StdInOutErr.
func (cl *CommandLog) Bytes() []byte {
	return cl.filterBytes(nil)
}

func (cl *CommandLog) BytesForBuildLogs() []byte {
	filtered := cl.filterBytes(nixBuildOutputPrefix)

	return bytes.TrimSpace(filtered) // Both prefix and suffix whitespace characters appear
}

// WriteString writes a string to the last line in StdInOutErr.
func (cl *CommandLog) WriteString(s string) (int, error) {
	return cl.Write([]byte(s))
}

// Write writes bytes to the last line in StdInOutErr.
func (cl *CommandLog) Write(data []byte) (int, error) {
	length := cl.Output.Length()
	if length == 0 {
		cl.Output.Append(safebuffer.NewBuffer(nil))

		length = 1
	}

	stdInOutErr, ok := cl.Output.Get(length - 1)
	if !ok {
		panic(fmt.Sprintf("internal error: command log %q: stdInOutErr index %d out of bounds (length=%d)", cl.Description, length-1, length))
	}

	written, err := stdInOutErr.Write(data)
	if err != nil {
		return written, errors.Wrap(err, "failed to write to command log")
	}

	return written, nil
}

// WriteLineString writes a string as a new line to StdInOutErr.
func (cl *CommandLog) WriteLineString(s string) {
	cl.WriteLine([]byte(s))
}

// WriteLine writes bytes as a new line to StdInOutErr.
func (cl *CommandLog) WriteLine(p []byte) {
	cl.Output.Append(safebuffer.NewBuffer(p))
}

// ReplaceLastLine replaces the content of the last line in StdInOutErr.
func (cl *CommandLog) ReplaceLastLine(data []byte) {
	if cl.Output.Length() == 0 {
		cl.WriteLine(data)

		return
	}

	index := cl.Output.Length() - 1
	ok := cl.Output.Set(index, safebuffer.NewBuffer(data))

	if !ok {
		panic(fmt.Sprintf("internal error: command log %q: failed to replace line at index %d (length=%d)", cl.Description, index, cl.Output.Length()))
	}
}

func (cl *CommandLog) filterBytes(skipPrefix []byte) []byte {
	values := cl.Output.Values()
	if len(values) == 0 {
		return nil
	}

	var result bytes.Buffer

	result.Grow(estimateSize(values))

	first := true

	for _, buf := range values {
		data := buf.Bytes()
		if skipPrefix != nil && bytes.HasPrefix(data, skipPrefix) {
			continue
		}

		if !first {
			result.WriteByte('\n')
		}

		first = false

		result.Write(data)
	}

	if result.Len() == 0 {
		return nil
	}

	return result.Bytes()
}

// Helpers

// estimateSize approximates the total size needed for all buffers.
func estimateSize(buffers []*safebuffer.Buffer) int {
	total := 0
	for _, buf := range buffers {
		total += buf.Len() + 1 // +1 for potential newline
	}

	return total
}
