package flake

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/pkg/errors"
)

//nolint:lll
type Flake struct {
	attributes.Attributes `yaml:",inline"`

	Nix            nix.NixConfig                                                            `yaml:"nix" json:"nix" desc:"Nix build and copy configuration"`
	URL            string                                                                   `yaml:"url" json:"url" validate:"omitempty,uri|dir" default:"." desc:"Flake path (eg. './dir') or url (eg. 'path:...', 'ssh:...', 'github:...'), reference https://nix.dev/manual/nix/2.33/command-ref/new-cli/nix3-flake.html#url-like-syntax"`
	Configurations *atomicorderedmap.AtomicOrderedMap[string, *configuration.Configuration] `yaml:"configurations,required" json:"configurations" validate:"required" desc:"Configurations in flake"`

	// Internal
	Logs *logs.Logs `yaml:"-" json:"logs,omitempty"`
}

func (f *Flake) Init(name string, attr *attributes.Attributes) error {
	err := f.Attributes.Init(name, attr)
	if err != nil {
		return errors.Wrap(err, "failed to initialize flake")
	}

	f.Nix.Init()

	if f.URL == "" {
		f.URL = "."
	}

	f.Logs = logs.New()

	return nil
}
