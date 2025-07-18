package executioner

import (
	"fmt"

	"github.com/mihakrumpestar/panix/internal/config"
)

// PingStream runs “nc -zvw1 host port” (or no‐op if local) and
// streams back its stdout/stderr in ShellEvent.  Alias lookup
// errors are reported as a single event with Err set.
func (ex *Executioner) PingStream(onFailure func(*config.Log, error) error, onSuccess func(*config.Log)) error {
	// 1) local short‐circuit
	if ex.local {
		exm := &config.CommandLog{
			Command: "ping (local) → skipped",
		}

		if ex.log.Commands == nil {
			ex.log.Commands = make([]*config.CommandLog, 0)
		}
		ex.log.Commands = append(ex.log.Commands, exm)
		if onSuccess != nil {
			onSuccess(ex.log)
		}
		ex.onUpdateHook()
		return nil
	}

	// 2) build nc args
	args := []string{"-zvw1"}

	host := ex.machineName.Hostname()
	if !ex.usesAlias {
		host = ex.machineSshConfig.Url.Host
	}
	args = append(args, host, fmt.Sprintf("%s", ex.machineSshConfig.Url.Port()))

	// 3) delegate to shellStream
	return ex.shellStream(onFailure, onSuccess, "nc", args...)
}
