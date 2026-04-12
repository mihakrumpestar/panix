package flake

import (
	config_attributes "github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/pkg/logs/phase"
	"github.com/mihakrumpestar/panix/internal/pkg/orderedmap"
	"github.com/pkg/errors"
)

type Flake struct {
	config_attributes.Attributes `yaml:",inline"`

	Configurations orderedmap.OrderedMap[string, *configuration.Configuration] `yaml:"configurations,required" json:"configurations" validate:"required"`
	URL            string                                                      `yaml:"url,required" json:"url" validate:"required,uri" desc:"Flake path (eg. 'path:...') or url (eg. 'ssh:...' 'github:...'), reference https://nix.dev/manual/nix/2.33/command-ref/new-cli/nix3-flake.html#url-like-syntax"` //nolint:lll

	// Internal
	Logs *logs.Logs `yaml:"-" json:"logs,omitempty"`
}

func (f *Flake) Init(name string, attr *config_attributes.Attributes) error {
	err := f.Attributes.Init(name, attr, false)
	if err != nil {
		return errors.Wrap(err, "failed to initialize flake")
	}

	f.Logs.PhaseLogs, err = phase.NewPhaseLogs(f.Xpath)
	if err != nil {
		return errors.Wrap(err, "failed to initialize flake logs")
	}

	return nil
}
