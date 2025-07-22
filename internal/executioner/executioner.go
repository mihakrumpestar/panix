package executioner

import (
	"context"
	"net/url"
	"strings"

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

func (ex *Executioner) Exec(skippable bool, onFailure func(*config.Log, error) error, onSuccess func(*config.Log) error, name string, args ...string) error {
	defer ex.onUpdateHook()

	// 1) local short‐circuit
	if ex.local && skippable {
		ex.log.AddMessageOnly("(skipped) ", name, " ", strings.Join(args, " "))
		if onSuccess != nil {
			err := onSuccess(ex.log)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if ex.local {
		return ex.shellStream(onFailure, onSuccess, name, args...)
	} else {
		return ex.sshStream(onFailure, onSuccess, name, args...)
	}
}
