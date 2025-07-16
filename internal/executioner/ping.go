package executioner

import (
	"fmt"
)

// PingStream runs “nc -zvw1 host port” (or no‐op if local) and
// streams back its stdout/stderr in ShellEvent.  Alias lookup
// errors are reported as a single event with Err set.
func (ex *Executioner) PingStream(onFailure func(*BaseMetadata, error) error, onSuccess func(*BaseMetadata)) error {
	// 1) local short‐circuit
	if ex.local {
		exm := &ExecutionerMetadata{
			Command: "ping (local) → skipped",
		}
		ex.meta.CommandOutputs = append(ex.meta.CommandOutputs, exm)
		if onSuccess != nil {
			onSuccess(ex.meta)
		}
		ex.onUpdateHook()
		return nil
	}

	// 2) build nc args
	args := []string{"-zvw1"}

	host := ex.meta.MachineName.Hostname()
	if !ex.usesAlias {
		host = ex.sshConfig.Url.Host
	}
	args = append(args, host, fmt.Sprintf("%s", ex.sshConfig.Url.Port()))

	// 3) delegate to shellStream
	return ex.shellStream(onFailure, onSuccess, "nc", args...)
}
