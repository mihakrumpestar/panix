package configuration

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/pkg/errors"
)

type Configuration struct {
	attributes.Attributes `yaml:",inline"`

	Machines    atomicorderedmap.AtomicOrderedMap[string, *machine.Machine] `yaml:"machines,required" json:"machines" validate:"required"`
	FlakeOutput string                                                      `yaml:"flake_output" json:"flake_output,omitempty" desc:"Override flake output (default: nixosConfigurations.<name>.config.system.build.toplevel)"` //nolint:lll
	// Internal
	MetaBuild *MetaBuild `yaml:"-" json:"meta_build,omitempty" validate:"-"`
	Logs      *logs.Logs `yaml:"-" json:"logs,omitempty"`
}

type MetaBuild struct {
	SystemClosure string `json:"system_closure,omitempty"`
}

func (c *Configuration) Init(name string, parentAttributes *attributes.Attributes) error {
	err := c.Attributes.Init(name, parentAttributes, false)
	if err != nil {
		return errors.Wrap(err, "failed to init configuration attributes")
	}

	c.Logs = logs.New()

	return nil
}
