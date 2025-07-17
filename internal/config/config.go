package config

import (
	"net/url"
	"time"

	"github.com/elliotchance/orderedmap/v3"
	"github.com/mihakrumpestar/panix/internal/workflow/workflow_definition"
)

type Config struct {
	Global Global                                 `koanf:"global"`
	Flakes *orderedmap.OrderedMap[string, *Flake] `koanf:"flakes"`
}

type Global struct {
	Filters           Filters                             `koanf:"filters"`
	RequireAllSuccess bool                                `koanf:"requireAllSuccess"`
	AutoBootstrap     bool                                `koanf:"autoBootstrap"`
	LocalMachine      string                              `koanf:"localMachine"`
	DryRun            bool                                `koanf:"dryRun"`
	Ssh               *Ssh                                `koanf:"ssh"`
	Timeout           time.Duration                       `koanf:"timeout"` // Time in seconds (int initialy, but we multiply by seconds later)
	Concurrency       int                                 `koanf:"concurrency"`
	SkipPhases        []workflow_definition.WorkflowPhase `koanf:"skipPhases"`
	Verbose           bool                                `koanf:"verbose"`
	Debug             bool                                `koanf:"debug"`
	//Json              bool                                `koanf:"json"` // Maybe later
}

type Filters struct {
	Flakes         []string `koanf:"flakes"`
	Configurations []string `koanf:"configurations"`
	Machines       []string `koanf:"machines"`
	Tags           []string `koanf:"tags"`
}

type Flake struct {
	Url             string `koanf:"url"` // Flake path or url
	treeStyleParams `koanf:",squash"`
	Configurations  *orderedmap.OrderedMap[string, *Configuration] `koanf:"configurations"`
}

type Configuration struct {
	FlakeOutput     string `koanf:"flakeOutput"` // Override if not standard style
	treeStyleParams `koanf:",squash"`
	Machines        *orderedmap.OrderedMap[url.URL, *Machine] `koanf:"machines"` // Key here is the ssh URL: alias, user@host or user@host:port
}

type Machine struct {
	treeStyleParams `koanf:",squash"`
}

type treeStyleParams struct {
	Ssh      *Ssh           `koanf:"ssh,omitempty"`
	Tags     []string       `koanf:"tags"`
	Secrets  []SecretConfig `koanf:"secrets,omitempty"`
	Disabled bool           `koanf:"disabled"`
}

type Ssh struct {
	Url        *url.URL `koanf:"-"`
	PrivateKey string   `koanf:"privateKey"`
	PublicKey  string   `koanf:"publicKey"`
}

type SecretConfig struct {
	LocalPath  string `koanf:"localPath"`
	RemotePath string `koanf:"remotePath"`
	Mode       string `koanf:"mode"`
}
