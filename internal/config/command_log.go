package config

import (
	"bytes"
	"os"
	"sync"

	"go.uber.org/atomic"
)

type CommandLog struct {
	Command      atomic.String
	mutex        sync.Mutex
	stdInOutErr  []*bytes.Buffer // Each line is a separate buffer to allow line replacement
	TimeAndState *TimeAndState
	Pty          *os.File
}

func NewCommandLog() *CommandLog {
	return &CommandLog{
		stdInOutErr:  make([]*bytes.Buffer, 0),
		TimeAndState: NewTimeAndState(),
	}
}

// Bytes wrapper
func (cl *CommandLog) String() string {
	return string(cl.Bytes())
}

// Bytes returns the byte representation of all lines in StdInOutErr
func (cl *CommandLog) Bytes() []byte {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	var result bytes.Buffer
	for i, buf := range cl.stdInOutErr {
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
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	// If there are no lines, create the first one
	if len(cl.stdInOutErr) == 0 {
		cl.stdInOutErr = append(cl.stdInOutErr, bytes.NewBuffer(nil))
	}

	return cl.stdInOutErr[len(cl.stdInOutErr)-1].Write(p)
}

// WriteLine wrapper
func (cl *CommandLog) WriteLineString(s string) {
	cl.WriteLine([]byte(s))
}

// WriteLine writes a new line to StdInOutErr
func (cl *CommandLog) WriteLine(p []byte) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	cl.stdInOutErr = append(cl.stdInOutErr, bytes.NewBuffer(p))
}

// ReplaceLastLine replaces the content of the last line in StdInOutErr
func (cl *CommandLog) ReplaceLastLine(p []byte) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	if len(cl.stdInOutErr) == 0 {
		cl.stdInOutErr = append(cl.stdInOutErr, bytes.NewBuffer(p))
	} else {
		cl.stdInOutErr[len(cl.stdInOutErr)-1] = bytes.NewBuffer(p)
	}
}
