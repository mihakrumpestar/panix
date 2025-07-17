package executioner

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/mihakrumpestar/panix/internal/config"
)

type Executioner struct {
	ctx          context.Context
	meta         *BaseMeta
	onUpdateHook func()
	local        bool
	dryRun       bool
	usesAlias    bool
	sshConfig    *config.Ssh
}

type BaseMeta struct {
	MetadataID
	PhaseMeta
}

type MetadataID struct {
	FlakeName         string
	ConfigurationName string
	MachineName       *url.URL
}

type PhaseMeta struct {
	CommandOutputs []*ExecutionerMetadata
	TimeSE
	Error error
}

type ExecutionerMetadata struct {
	Command     string
	Stdout      strings.Builder
	Stderr      strings.Builder
	StdCombined strings.Builder
	TimeSE
	Error error
}

type TimeSE struct {
	StartTime time.Time
	EndTime   time.Time
}

func New(ctx context.Context, meta *BaseMeta, onUpdateHook func(), c *config.Global, machine *config.Machine) *Executioner {
	if meta == nil {
		panic("meta is not allowe to be nil here")
	}

	local := true // No machine means we are doing building (which is currently local only)
	usesAlias := false
	var sshConfig *config.Ssh

	if machine != nil {
		local = c.LocalMachine == meta.MachineName.String()
		usesAlias = meta.MachineName.User.String() == ""
		sshConfig = machine.Ssh
	}

	return &Executioner{
		ctx:          ctx,
		meta:         meta,
		onUpdateHook: onUpdateHook,
		local:        local,
		dryRun:       c.DryRun,
		usesAlias:    usesAlias,
		sshConfig:    sshConfig,
	}
}

func (ex *Executioner) Exec(onFailure func(*BaseMeta, error) error, onSuccess func(*BaseMeta), name string, args ...string) error {
	if ex.local {
		return ex.shellStream(onFailure, onSuccess, name, args...)
	} else {
		return ex.sshStream(onFailure, onSuccess, name, args...)
	}
}
