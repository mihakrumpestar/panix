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
	User           string                                                       `yaml:"user" json:"user,omitempty" desc:"Target user for activation. When set, commands run as this user via su -l, which requires SSHing as root (su -l would prompt for a password otherwise; system-level types elevate each command with the sudo program unless the target user is root, which is the default; a non-root target user needs passwordless sudo). If empty, uses the SSH username."`
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

// RemoteBuilder pins remote builds to the first declared machine (post-filter
// order), used by both build (--store) and transfer (--from); validation
// guarantees remote-mode installables have one, so nil means an invalid config.
func (i *Installable) RemoteBuilder() *machine.Machine {
	for _, pair := range i.Machines.Pairs() {
		if pair.Value != nil {
			return pair.Value
		}
	}

	return nil
}

// Init takes type/name from the YAML keys, registers both as tags, and uses a
// composite type/name xpath so same-named outputs of different types cannot
// collide. Preset defaults come from the built-in table for known types or
// from the output_types declaration for custom ones.
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

	i.Attributes.Name = stringbyte.StringByte(nameKey)
	if len(i.Attributes.Tags) > 0 {
		i.Attributes.Tags[len(i.Attributes.Tags)-1] = nameKey
	}

	i.Attributes.Tags = append(i.Attributes.Tags, typeKey.String())

	err = i.Nix.Init(parentNix)
	if err != nil {
		return errors.Wrap(err, "failed to initialize installable nix config")
	}

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

// applyPresetDefaults merges type defaults into the Preset: user-overridable
// fields keep a non-zero user value, while type-level fields are intrinsic to
// the output type and always come from defaults.
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

	if len(i.Preset.NonMutatingModes) == 0 {
		i.Preset.NonMutatingModes = defaults.NonMutatingModes
	}

	if len(i.Preset.ProfileSkipModes) == 0 {
		i.Preset.ProfileSkipModes = defaults.ProfileSkipModes
	}

	if i.Preset.ActivationDefaultMode == "" {
		i.Preset.ActivationDefaultMode = defaults.ActivationDefaultMode
	}

	i.Preset.IsSystemLevel = defaults.IsSystemLevel
	i.Preset.IsBootstrappable = defaults.IsBootstrappable
	i.Preset.OmitTypeFromAttrPath = defaults.OmitTypeFromAttrPath
}
