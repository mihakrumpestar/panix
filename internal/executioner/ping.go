package executioner

import (
	"fmt"
	"strings"
)

// PingStream runs “nc -zvw1 host port” (or no‐op if local) and
// streams back its stdout/stderr in ShellEvent.  Alias lookup
// errors are reported as a single event with Err set.
func (ex *Executioner) PingStream() <-chan ExecutionerOutput {
	ch := make(chan ExecutionerOutput)

	go func() {
		defer close(ch)

		// 1) local short‐circuit
		if ex.local {
			ch <- ExecutionerOutput{
				Command: "ping (local) → skipped",
				Stdout:  strings.Builder{},
				Stderr:  strings.Builder{},
			}
			return
		}

		// 2) build nc args
		args := []string{"-zvw1"}

		host := ex.machineName.Host
		if !ex.usesAlias {
			host = ex.sshConfig.Url.Host
		}
		args = append(args, host, fmt.Sprintf("%d", ex.sshConfig.Url.Port))

		// 3) delegate to shellStream
		//
		// shellStream will send an initial event, then stream any
		// stdout/stderr lines, and finally one last event with Err
		// set on failure (or just a final success event).
		for evt := range ex.shellStream("nc", args...) {
			ch <- evt
		}
	}()

	return ch
}
