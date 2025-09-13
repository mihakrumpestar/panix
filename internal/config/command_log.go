package config

import (
	"bytes"
	"os"
	"strings"
)

type CommandLog struct {
	Command      string
	StdInOutErr  []*bytes.Buffer // Each line is a separate buffer to allow line replacement
	TimeAndState TimeAndState
	Pty          *os.File
}

// String returns the string representation of all lines in StdInOutErr
func (cl *CommandLog) String() string {
	var result strings.Builder
	for i, buf := range cl.StdInOutErr {
		if i > 0 {
			result.WriteString("\n")
		}
		result.Write(buf.Bytes())
	}
	return result.String()
}

// Bytes returns the byte representation of all lines in StdInOutErr
func (cl *CommandLog) Bytes() []byte {
	var result bytes.Buffer
	for i, buf := range cl.StdInOutErr {
		if i > 0 {
			result.WriteByte('\n')
		}
		result.Write(buf.Bytes())
	}
	return result.Bytes()
}

// WriteString writes a string to the last line in StdInOutErr
func (cl *CommandLog) WriteString(s string) (int, error) {
	// If there are no lines, create the first one
	if len(cl.StdInOutErr) == 0 {
		cl.StdInOutErr = append(cl.StdInOutErr, bytes.NewBuffer(nil))
	}
	return cl.StdInOutErr[len(cl.StdInOutErr)-1].WriteString(s)
}

// Write writes bytes to the last line in StdInOutErr
func (cl *CommandLog) Write(p []byte) (int, error) {
	// If there are no lines, create the first one
	if len(cl.StdInOutErr) == 0 {
		cl.StdInOutErr = append(cl.StdInOutErr, bytes.NewBuffer(nil))
	}
	return cl.StdInOutErr[len(cl.StdInOutErr)-1].Write(p)
}

// WriteLine writes a new line to StdInOutErr
func (cl *CommandLog) WriteLine(p []byte) {
	cl.StdInOutErr = append(cl.StdInOutErr, bytes.NewBuffer(p))
}

func (cl *CommandLog) WriteLineString(s string) {
	cl.WriteLine([]byte(s))
}

// ReplaceLastLine replaces the content of the last line in StdInOutErr
func (cl *CommandLog) ReplaceLastLine(p []byte) {
	if len(cl.StdInOutErr) == 0 {
		cl.StdInOutErr = append(cl.StdInOutErr, bytes.NewBuffer(p))
	} else {
		cl.StdInOutErr[len(cl.StdInOutErr)-1] = bytes.NewBuffer(p)
	}
}
