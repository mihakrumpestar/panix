package executioner

import (
	"bytes"
	"context"

	"github.com/mihakrumpestar/panix/internal/config"
)

type Executioner struct {
	ctx       context.Context
	local     bool
	sshConfig config.Ssh
}

type ExecutionerOutput struct {
	Command string
	Stdout  bytes.Buffer
	Stderr  bytes.Buffer
}

func New(ctx context.Context, local bool, ssh config.Ssh) (*Executioner, error) {
	return &Executioner{
		ctx:       ctx,
		local:     local,
		sshConfig: ssh,
	}, nil
}

func (ex *Executioner) Exec(name string, args ...string) (ExecutionerOutput, error) {
	if ex.local {
		return ex.shell(name, args...)
	} else {
		return ex.ssh(name, args...)
	}
}
