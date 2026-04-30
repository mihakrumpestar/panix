package flake

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/pkg/errors"
)

//nolint:lll
type Flake struct {
	attributes.Attributes `yaml:",inline"`

	Nix            nix.NixConfig                                                           `yaml:"nix" json:"nix" desc:"Nix build and copy configuration"`
	URL            string                                                                   `yaml:"url,required" json:"url" validate:"required,uri" desc:"Flake path (eg. 'path:...') or url (eg. 'ssh:...' 'github:...'), reference https://nix.dev/manual/nix/2.33/command-ref/new-cli/nix3-flake.html#url-like-syntax"`
	Configurations *atomicorderedmap.AtomicOrderedMap[string, *configuration.Configuration] `yaml:"configurations,required" json:"configurations" validate:"required" desc:"Configurations in flake"`

	// Internal
	Logs *logs.Logs `yaml:"-" json:"logs,omitempty"`
}

func (f *Flake) Init(name string, attr *attributes.Attributes, localMachineHostname string) error {
	err := f.Attributes.Init(name, attr, false, localMachineHostname)
	if err != nil {
		return errors.Wrap(err, "failed to initialize flake")
	}

	f.Nix.Init()
	f.Logs = logs.New()

	return nil
}
