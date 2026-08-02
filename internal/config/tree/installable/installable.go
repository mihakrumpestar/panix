package installable

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/stringbyte"
	"github.com/pkg/errors"
)

//nolint:lll
type Installable struct {
	attributes.Attributes `yaml:",inline"`

	Nix            nix.NixConfig                                                `yaml:"nix" json:"nix" desc:"Nix build and copy configuration"`
	Type           FlakeOutputType                                              `yaml:"-" json:"type" desc:"Flake output type (e.g. nixosConfigurations, homeConfigurations)"`
	Name           AttributeName                                                `yaml:"-" json:"name" desc:"Attribute name (e.g. server1, alice)"`
	ActivationMode attributes.ActivationModeD                                   `yaml:"activation_mode" json:"activation_mode,omitempty" desc:"Activation mode: check, switch, boot, test, dry-activate (only for nixosConfigurations)" default:"switch" validate:"omitempty,oneof=check switch boot test dry-activate"`
	Machines       *atomicorderedmap.AtomicOrderedMap[string, *machine.Machine] `yaml:"machines,required" json:"machines" validate:"required" desc:"Machines configuration" schema:"nullable_values"`

	// Internal
	MetaBuild *MetaBuild `yaml:"-" json:"meta_build,omitempty"`
	Logs      *logs.Logs `yaml:"-" json:"logs,omitempty"`
}

type MetaBuild struct {
	Closure string `yaml:"-" json:"closure,omitempty"`
}

// Init initializes the Installable, merging parent attributes and nix config.
// The type and name are set from the YAML keys (the two-level map key).
// The xpath uses the composite key (type/name) to avoid collisions between
// outputs with the same attribute name but different types
// (e.g. nixosConfigurations/server1 vs homeConfigurations/server1).
// Both the output type and attribute name are registered as tags.
func (i *Installable) Init(typeKey FlakeOutputType, nameKey string, parentAttributes *attributes.Attributes, parentNix *nix.NixConfig) error {
	i.Type = typeKey
	i.Name = AttributeName(nameKey)

	compositeKey := CompositeKey(typeKey, AttributeName(nameKey))

	err := i.Attributes.Init(compositeKey, parentAttributes)
	if err != nil {
		return errors.Wrap(err, "failed to init installable attributes")
	}

	// Init appended the composite key as a tag. Replace it with the
	// attribute name, and also append the output type as a separate tag.
	i.Attributes.Name = stringbyte.StringByte(nameKey)
	if len(i.Attributes.Tags) > 0 {
		i.Attributes.Tags[len(i.Attributes.Tags)-1] = nameKey
	}

	i.Attributes.Tags = append(i.Attributes.Tags, typeKey.String())

	err = i.Nix.Init(parentNix)
	if err != nil {
		return errors.Wrap(err, "failed to initialize installable nix config")
	}

	i.Logs = logs.New()

	return nil
}
