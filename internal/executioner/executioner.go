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
	meta         *BaseMetadata
	onUpdateHook func()
	local        bool
	dryRun       bool
	usesAlias    bool
	sshConfig    *config.Ssh
}

type BaseMetadata struct {
	MetadataID
	PhaseMetadata
}

type MetadataID struct {
	FlakeName         string
	ConfigurationName string
	MachineName       *url.URL
}

type PhaseMetadata struct {
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

func New(ctx context.Context, meta *BaseMetadata, onUpdateHook func(), c *config.Global, machine *config.Machine) *Executioner {
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

func (ex *Executioner) Exec(onFailure func(*BaseMetadata, error) error, onSuccess func(*BaseMetadata), name string, args ...string) error {
	if ex.local {
		return ex.shellStream(onFailure, onSuccess, name, args...)
	} else {
		return ex.sshStream(onFailure, onSuccess, name, args...)
	}
}
