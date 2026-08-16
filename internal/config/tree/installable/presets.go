package installable

import (
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
)

// CustomOutputTypes maps user-declared custom flake output type names to their
// presets. Loaded from the top-level output_types section of panix.yml and
// applied as defaults to installables of that type, with the same merge
// semantics as built-in presets (installable-level YAML overrides win).
type CustomOutputTypes = *atomicorderedmap.AtomicOrderedMap[string, Preset]

// Preset defines the build path, activation mechanism, and profile management
// for a specific FlakeOutputType. Each known output type has a preset that
// auto-infers behavior, users don't need to specify these manually.
//
// Fields are split into two categories:
//   - User-overridable: settable per-installable via YAML. Zero values fall
//     back to the type default. (OutputTypeAttr, BuildPath, ProfilePath,
//     ActivationPath, SetProfile, ActivationModes, ActivationDefaultMode,
//     NonMutatingModes, ProfileSkipModes)
//   - Type-level: intrinsic to the output type, not user-configurable
//     per-installable. Always taken from the type default. For built-ins the
//     default comes from the presets table; for custom types declared under
//     output_types it comes from the declared preset. (IsSystemLevel,
//     IsBootstrappable, OmitTypeFromAttrPath)
//
//nolint:lll
type Preset struct {
	// User-overridable fields.
	OutputTypeAttr        string   `yaml:"output_type_attr,omitempty" json:"output_type_attr,omitempty" desc:"Top-level flake output attribute used in the attrpath (defaults to the output type key)"`
	BuildPath             string   `yaml:"build_path,omitempty" json:"build_path,omitempty" desc:"Build path within the flake output"`
	ProfilePath           string   `yaml:"profile_path,omitempty" json:"profile_path,omitempty" desc:"Nix profile path"`
	ActivationPath        string   `yaml:"activation_path,omitempty" json:"activation_path,omitempty" desc:"Activation script path in the closure"`
	SetProfile            *bool    `yaml:"set_profile,omitempty" json:"set_profile,omitempty" desc:"Run nix-env --profile --set before activation"`
	ActivationModes       []string `yaml:"activation_supported_modes,omitempty" json:"activation_supported_modes,omitempty" desc:"Activation modes"`
	NonMutatingModes      []string `yaml:"activation_non_mutating_modes,omitempty" json:"activation_non_mutating_modes,omitempty" desc:"Activation modes that do not mutate the target system (auto rollback is skipped for these modes)"`
	ProfileSkipModes      []string `yaml:"activation_profile_skip_modes,omitempty" json:"activation_profile_skip_modes,omitempty" desc:"Activation modes that skip setting the profile before activation (the activation runs against the passed closure without switching the profile to it)"`
	ActivationDefaultMode string   `yaml:"activation_default_mode,omitempty" json:"activation_default_mode,omitempty" desc:"Default activation mode"`

	// Type-level fields. For built-in types these are not user-configurable
	// (always taken from the type default). For custom types declared under
	// output_types they are set in the declaration and still applied as type
	// defaults to every installable of that type.
	IsSystemLevel        *bool `yaml:"system_level,omitempty" json:"system_level,omitempty" desc:"System-level (root) vs user-level. Type-level field: set only under output_types declarations"`
	IsBootstrappable     bool  `yaml:"-" json:"-" desc:"Supports bootstrap"`
	OmitTypeFromAttrPath bool  `yaml:"omit_type_from_attr_path,omitempty" json:"omit_type_from_attr_path,omitempty" desc:"Omit output type from attrpath (for packages where nix auto-resolves bare names). Type-level field: set only under output_types declarations"`
}

// IsSystemLevelValue reports whether the preset targets the system level (root) rather than a user level.
func (p Preset) IsSystemLevelValue() bool {
	return p.IsSystemLevel != nil && *p.IsSystemLevel
}

// presets maps each known FlakeOutputType to its Preset.
var presets = map[FlakeOutputType]Preset{
	FlakeOutputType("nixosConfigurations"): {
		BuildPath:             "config.system.build.toplevel",
		ProfilePath:           "/nix/var/nix/profiles/system",
		SetProfile:            new(true),
		IsSystemLevel:         new(true),
		ActivationPath:        "bin/switch-to-configuration",
		ActivationModes:       []string{"switch", "boot", "test", "dry-activate"},
		ProfileSkipModes:      []string{"test", "dry-activate"},
		NonMutatingModes:      []string{"dry-activate"},
		ActivationDefaultMode: "switch",
		IsBootstrappable:      true,
	},
	FlakeOutputType("darwinConfigurations"): {
		BuildPath:      "system",
		ProfilePath:    "/nix/var/nix/profiles/system",
		SetProfile:     new(true),
		IsSystemLevel:  new(true),
		ActivationPath: "activate",
	},
	FlakeOutputType("systemConfigs"): {
		BuildPath:      "",
		ProfilePath:    "/nix/var/nix/profiles/system-manager-profiles",
		SetProfile:     new(true),
		IsSystemLevel:  new(true),
		ActivationPath: "bin/activate",
	},
	FlakeOutputType("homeConfigurations"): {
		BuildPath:      "activationPackage",
		ProfilePath:    "~/.local/state/nix/profiles/home-manager",
		IsSystemLevel:  new(false),
		ActivationPath: "activate",
	},
	FlakeOutputType("nixOnDroidConfigurations"): {
		BuildPath:      "build.activationPackage",
		ProfilePath:    "~/.local/state/nix/profiles/nix-on-droid",
		IsSystemLevel:  new(false),
		ActivationPath: "activate",
	},
	FlakeOutputType("packages"): {
		BuildPath:            "",
		IsSystemLevel:        new(false),
		OmitTypeFromAttrPath: true,
	},
	// User-defined output convention for nix-maid (https://github.com/viperML/nix-maid)
	// configuration bundles; activated via bin/activate in the closure.
	FlakeOutputType("maidConfigurations"): {
		IsSystemLevel:  new(false),
		ActivationPath: "bin/activate",
	},
}

var knownOutputTypes = func() []FlakeOutputType {
	types := make([]FlakeOutputType, 0, len(presets))
	for t := range presets {
		types = append(types, t)
	}

	return types
}()

func KnownOutputTypes() []FlakeOutputType {
	return knownOutputTypes
}

func (t FlakeOutputType) IsKnown() bool {
	_, ok := presets[t]

	return ok
}

func IsBootstrappableType(t FlakeOutputType) bool {
	p, ok := presets[t]
	if !ok {
		return false
	}

	return p.IsBootstrappable
}

// ActivationModes returns the activation modes supported by nixosConfigurations.
func ActivationModes() []string {
	return presets[FlakeOutputType("nixosConfigurations")].ActivationModes
}
