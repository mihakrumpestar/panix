package command

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
)

type AtomicCommandOutput struct {
	mutex sync.Mutex

	// Content is StdIn, StdOut, and StdErr, each line is a separate buffer to allow efficient line replacement
	content []*bytes.Buffer
	version uint64
}

func NewAtomicCommandOutput() *AtomicCommandOutput {
	return &AtomicCommandOutput{
		content: make([]*bytes.Buffer, 0),
	}
}

// Version returns the current version counter (incremented on every mutation).
func (ao *AtomicCommandOutput) Version() uint64 {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	return ao.version
}

// String returns the string representation of all lines in content.
func (ao *AtomicCommandOutput) String() string {
	return string(ao.Bytes())
}

// Bytes returns the byte representation of all lines in content.
func (ao *AtomicCommandOutput) Bytes() []byte {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	var result bytes.Buffer

	first := true

	for _, buf := range ao.content {
		if !first {
			result.WriteByte('\n')
		}

		first = false

		result.Write(buf.Bytes())
	}

	if result.Len() == 0 {
		return nil
	}

	return result.Bytes()
}

// Write writes bytes to the last line in content.
func (ao *AtomicCommandOutput) Write(buf []byte) {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	ao.version++

	if len(ao.content) == 0 {
		ao.content = append(ao.content, bytes.NewBuffer(buf))

		return
	}

	_, _ = ao.lastLineUnsafe().Write(buf)
}

// WriteLineString writes a string as a new line to content.
func (ao *AtomicCommandOutput) WriteLineString(s string) {
	ao.WriteLine([]byte(s))
}

// WriteLine writes bytes as a new line to content.
func (ao *AtomicCommandOutput) WriteLine(p []byte) {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	ao.content = append(ao.content, bytes.NewBuffer(p))
	ao.version++
}

// TrimTrailingEmptyLines removes empty lines from the end of the output.
// PTY output typically ends with a trailing newline, which processSequence
// converts into an empty line entry. This strips those artifact empty lines
// while preserving intentional blank lines mid-output.
func (ao *AtomicCommandOutput) TrimTrailingEmptyLines() {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	changed := false

	for len(ao.content) > 0 {
		lastLine := ao.lastLineUnsafe()
		if lastLine.Len() > 0 {
			break
		}

		ao.content = ao.content[:len(ao.content)-1]
		changed = true
	}

	if changed {
		ao.version++
	}
}

// ReplaceLastLine replaces the content of the last line in content.
func (ao *AtomicCommandOutput) ReplaceLastLine(data []byte) {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	ao.version++

	if len(ao.content) == 0 {
		ao.content = append(ao.content, bytes.NewBuffer(data))

		return
	}

	lastLine := ao.lastLineUnsafe()
	lastLine.Reset()
	lastLine.Write(data)
}

// Len returns the number of lines in content.
func (ao *AtomicCommandOutput) Len() int {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	return len(ao.content)
}

func (ao *AtomicCommandOutput) MarshalJSON() ([]byte, error) {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	lines := make([]string, len(ao.content))
	for i, buf := range ao.content {
		lines[i] = buf.String()
	}

	out := struct {
		Lines   []string `json:"lines"`
		Version uint64   `json:"version"`
	}{
		Lines:   lines,
		Version: ao.version,
	}

	b, err := json.Marshal(out)

	return b, errors.Wrap(err, "marshal atomic command output")
}

func (ao *AtomicCommandOutput) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	dataStruct := struct {
		Lines   []string `json:"lines"`
		Version uint64   `json:"version"`
	}{}

	err := json.Unmarshal(data, &dataStruct)
	if err != nil {
		return errors.Wrap(err, "unmarshal atomic command output")
	}

	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	ao.content = make([]*bytes.Buffer, len(dataStruct.Lines))
	for i, line := range dataStruct.Lines {
		ao.content[i] = bytes.NewBufferString(line)
	}

	ao.version = dataStruct.Version

	return nil
}

// Helpers

// lastLineUnsafe returns last line from content. Must be called with mutex held.
func (ao *AtomicCommandOutput) lastLineUnsafe() *bytes.Buffer {
	length := len(ao.content)

	if length == 0 {
		panic("internal error: lastLineUnsafe is not designed to handle empty content slice")
	}

	return ao.content[length-1]
}
