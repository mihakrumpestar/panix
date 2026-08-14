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
	Preset         Preset                                                       `yaml:"preset" json:"preset"`
	User           string                                                       `yaml:"user" json:"user,omitempty" desc:"Target user for activation (user-level types only). When set, activation runs as this user via su -l. If empty, uses the SSH username."`
	ActivationMode string                                                       `yaml:"activation_mode" json:"activation_mode,omitempty" desc:"Activation mode (overrides preset default)"`
	Machines       *atomicorderedmap.AtomicOrderedMap[string, *machine.Machine] `yaml:"machines,required" json:"machines" validate:"required" desc:"Machines configuration" schema:"nullable_values"`

	// Internal
	Type      FlakeOutputType `yaml:"-" json:"type" desc:"Flake output type (e.g. nixosConfigurations, homeConfigurations)"`
	Name      AttributeName   `yaml:"-" json:"name" desc:"Attribute name (e.g. server1, alice)"`
	MetaBuild *MetaBuild      `yaml:"-" json:"meta_build,omitempty"`
	Logs      *logs.Logs      `yaml:"-" json:"logs,omitempty"`
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
//
// After setting the type, preset defaults for this output type are merged into
// the Preset's zero-value user-configurable fields. Type-level fields (not
// user-configurable) are always taken from the defaults.
//
// The defaults source is the built-in presets table for known types, or the
// custom preset declared under output_types (customPresets) for custom types.
// Both use the same merge semantics: installable-level YAML overrides win for
// user-overridable fields, type-level fields are always forced from defaults.
func (i *Installable) Init(
	typeKey FlakeOutputType,
	nameKey string,
	parentAttributes *attributes.Attributes,
	parentNix *nix.NixConfig,
	customPresets CustomOutputTypes,
) error {
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

	// Apply type defaults to zero-value preset fields. Built-in types take
	// their defaults from the presets table; custom types declared under
	// output_types use the same merge semantics with their declared preset.
	defaults, ok := presets[typeKey]
	if !ok && customPresets != nil {
		defaults, ok = customPresets.Get(typeKey.String())
	}

	if ok {
		i.applyPresetDefaults(defaults)
	}

	i.Logs = logs.New()

	return nil
}

// applyPresetDefaults merges type defaults into the Preset.
//
// User-overridable fields (OutputTypeAttr, BuildPath, ProfilePath, ActivationPath,
// SetProfile, ActivationModes, ActivationDefaultMode) use the user value if
// non-zero, otherwise fall back to the type default.
//
// Type-level fields (IsSystemLevel, IsBootstrappable, OmitTypeFromAttrPath)
// are intrinsic to the output type and always taken from the defaults,
// ignoring any user-provided value.
func (i *Installable) applyPresetDefaults(defaults Preset) {
	if i.Preset.OutputTypeAttr == "" {
		i.Preset.OutputTypeAttr = defaults.OutputTypeAttr
	}

	if i.Preset.BuildPath == "" {
		i.Preset.BuildPath = defaults.BuildPath
	}

	if i.Preset.ProfilePath == "" {
		i.Preset.ProfilePath = defaults.ProfilePath
	}

	if i.Preset.ActivationPath == "" {
		i.Preset.ActivationPath = defaults.ActivationPath
	}

	if i.Preset.SetProfile == nil {
		i.Preset.SetProfile = defaults.SetProfile
	}

	if len(i.Preset.ActivationModes) == 0 {
		i.Preset.ActivationModes = defaults.ActivationModes
	}

	if i.Preset.ActivationDefaultMode == "" {
		i.Preset.ActivationDefaultMode = defaults.ActivationDefaultMode
	}

	// Type-level fields — always from defaults, not user-configurable.
	i.Preset.IsSystemLevel = defaults.IsSystemLevel
	i.Preset.IsBootstrappable = defaults.IsBootstrappable
	i.Preset.OmitTypeFromAttrPath = defaults.OmitTypeFromAttrPath
}
