package executioner

import (
	"context"
	"net/url"

	"github.com/mihakrumpestar/panix/internal/config"
)

type Executioner struct {
	ctx              context.Context
	dryRun           bool
	local            bool
	usesAlias        bool
	machineName      *url.URL
	machineSshConfig *config.SshClient
	log              *config.Log
	onUpdateHook     func()
}

// NewExecutioner: if machineName == nil, machineSshConfig won't be used
func NewExecutioner(ctx context.Context, conf *config.Global, machineName *url.URL, machineSshConfig *config.SshClient, log *config.Log, onUpdateHook func()) *Executioner {
	local := true // No machine means we are doing the building phase (which is currently local only)
	usesAlias := false

	if machineName != nil {
		local = conf.LocalMachine == machineName.Hostname()
		usesAlias = machineName.User.String() == ""
	}

	return &Executioner{
		ctx:              ctx,
		dryRun:           conf.DryRun,
		local:            local,
		usesAlias:        usesAlias,
		machineName:      machineName,
		machineSshConfig: machineSshConfig,
		log:              log,
		onUpdateHook:     onUpdateHook,
	}
}

func (ex *Executioner) Exec(onFailure func(*config.Log, error) error, onSuccess func(*config.Log), name string, args ...string) error {
	if ex.local {
		return ex.shellStream(onFailure, onSuccess, name, args...)
	} else {
		return ex.sshStream(onFailure, onSuccess, name, args...)
	}
}
