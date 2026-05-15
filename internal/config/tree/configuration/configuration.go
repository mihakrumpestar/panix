package configuration

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/pkg/errors"
)

//nolint:lll
type Configuration struct {
	attributes.Attributes `yaml:",inline"`

	Nix         nix.NixConfig                                                `yaml:"nix" json:"nix" desc:"Nix build and copy configuration"`
	FlakeOutput FlakeOutput                                                  `yaml:"flake_output" json:"flake_output,omitempty" desc:"Nix flake output attribute (e.g. nixosConfigurations.<name>)" default:"nixosConfigurations.<name>"`
	BuildPath   BuildPath                                                    `yaml:"build_path" json:"build_path,omitempty" desc:"Build path within the flake output (e.g. config.system.build.toplevel)" default:"config.system.build.toplevel"`
	Machines    *atomicorderedmap.AtomicOrderedMap[string, *machine.Machine] `yaml:"machines,required" json:"machines" validate:"required" desc:"Machines configuration" schema:"nullable_values"`
	// Internal
	MetaBuild *MetaBuild `yaml:"-" json:"meta_build,omitempty"`
	Logs      *logs.Logs `yaml:"-" json:"logs,omitempty"`
}

type MetaBuild struct {
	SystemClosure string `yaml:"-" json:"system_closure,omitempty"`
}

func (c *Configuration) Init(name string, parentAttributes *attributes.Attributes, localMachineHostname string) error {
	err := c.Attributes.Init(name, parentAttributes, false, localMachineHostname)
	if err != nil {
		return errors.Wrap(err, "failed to init configuration attributes")
	}

	c.Nix.Init()
	c.Logs = logs.New()

	return nil
}
