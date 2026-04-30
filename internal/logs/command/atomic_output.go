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
// The last line is excluded if empty (PTY trailing newline artifact).
func (ao *AtomicCommandOutput) Bytes() []byte {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	lastLineIdx := len(ao.content) - 1

	var result bytes.Buffer

	for idx, buf := range ao.content {
		if idx == lastLineIdx && buf.Len() == 0 {
			break
		}

		if idx != 0 {
			result.WriteByte('\n')
		}

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

// Len returns the number of lines in content, excluding a trailing empty line.
func (ao *AtomicCommandOutput) Len() int {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	length := len(ao.content)

	if length > 0 && ao.content[length-1].Len() == 0 {
		length--
	}

	return length
}

func (ao *AtomicCommandOutput) MarshalJSON() ([]byte, error) {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	end := len(ao.content)

	if end > 0 && ao.content[end-1].Len() == 0 {
		end--
	}

	lines := make([]string, end)
	for i := range end {
		lines[i] = ao.content[i].String()
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
