package executioner

import (
	"context"
	"net/url"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config"
)

type Executioner struct {
	ctx         context.Context
	local       bool
	dryRun      bool
	usesAlias   bool
	machineName *url.URL
	sshConfig   *config.Ssh
}

type ExecutionerOutput struct {
	Command     string
	Stdout      strings.Builder
	Stderr      strings.Builder
	StdCombined strings.Builder
	Error       error
}

// ExecStep describes one streaming command in a pipeline.
type ExecStep struct {
	Stream    <-chan ExecutionerOutput
	OnInit    func()                      // OnInit is invoked for before ExecStep actually starts, so you may set or init things before the actualy execution.
	OnEvent   func(out ExecutionerOutput) // OnEvent is invoked for *every* ExecutionerOutput as it arrives.
	OnError   func(out ExecutionerOutput) // OnError is invoked *once* if the final ExecutionerOutput has Error != nil.
	OnSuccess func(out ExecutionerOutput) // OnSuccess is invoked *once* if the final ExecutionerOutput has Error == nil.
}

func New(ctx context.Context, c *config.Global, machineName *url.URL, machine *config.Machine) (*Executioner, error) {
	return &Executioner{
		ctx:         ctx,
		local:       !(machineName != nil && c.LocalMachine != machineName.String()),
		dryRun:      c.DryRun,
		usesAlias:   machineName != nil && machineName.User.String() == "",
		machineName: machineName,
		sshConfig:   machine.Ssh,
	}, nil
}

func (ex *Executioner) Exec(name string, args ...string) <-chan ExecutionerOutput {
	if ex.local {
		return ex.shellStream(name, args...)
	} else {
		return ex.sshStream(name, args...)
	}
}

// ExecBatch runs each step in order, streaming its outputs, firing
// hooks, and aborting early if any OnError returns true.
func (ex *Executioner) ExecBatch(steps ...ExecStep) {
	for _, step := range steps {
		step.OnInit()

		var last ExecutionerOutput
		for ev := range step.Stream {
			last = ev
			if step.OnEvent != nil {
				step.OnEvent(ev)
			}
		}
		if last.Error != nil {
			if step.OnError != nil {
				step.OnError(last)
				return
			}
		} else {
			if step.OnSuccess != nil {
				step.OnSuccess(last)
			}
		}
	}
}
