package executioner

import (
	"bytes"
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
)

type Executioner struct {
	ctx       context.Context
	local     bool
	dryRun    bool
	sshConfig *config.Ssh
}

type ExecutionerOutput struct {
	Command string
	Stdout  bytes.Buffer
	Stderr  bytes.Buffer
}

func New(ctx context.Context, dryRun bool, machine *config.Machine) (*Executioner, error) {
	return &Executioner{
		ctx:       ctx,
		local:     machine.Local,
		dryRun:    dryRun,
		sshConfig: machine.Ssh,
	}, nil
}

func (ex *Executioner) Exec(name string, args ...string) (ExecutionerOutput, error) {
	if ex.local {
		return ex.shell(name, args...)
	} else {
		return ex.ssh(name, args...)
	}
}
