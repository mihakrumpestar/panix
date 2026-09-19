package installable

import (
	"cmp"
	"slices"

	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
)

// CustomOutputTypes maps user-declared custom flake output type names to their
// presets, loaded from the top-level output_types section of panix.yml and
// applied with the same merge semantics as built-in presets.
type CustomOutputTypes = *atomicorderedmap.AtomicOrderedMap[string, Preset]

// Preset defines the build path, activation mechanism, and profile management
// for a FlakeOutputType. Fields come in three kinds: user-overridable (a
// non-zero per-installable value wins), type-level (intrinsic to the output
// type, always from the default) and docs-only (never read from or written to
// config).
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

	// Type-level fields: not user-configurable, always taken from the type
	// default (for custom types, from their output_types declaration).
	IsSystemLevel        *bool `yaml:"system_level,omitempty" json:"system_level,omitempty" desc:"System-level (root) vs user-level. Type-level field: set only under output_types declarations"`
	IsBootstrappable     bool  `yaml:"-" json:"-" desc:"Supports bootstrap"`
	OmitTypeFromAttrPath bool  `yaml:"omit_type_from_attr_path,omitempty" json:"omit_type_from_attr_path,omitempty" desc:"Omit output type from attrpath (for packages where nix auto-resolves bare names). Type-level field: set only under output_types declarations"`

	// Docs-only presentation metadata for the generated output-type tables;
	// custom types leave these empty and are never listed.
	DocOrder      int    `yaml:"-" json:"-"`
	DocDeploys    string `yaml:"-" json:"-"`
	DocActivation string `yaml:"-" json:"-"`
}

// IsSystemLevelValue treats nil (not declared) as user level.
func (p Preset) IsSystemLevelValue() bool {
	return p.IsSystemLevel != nil && *p.IsSystemLevel
}

//nolint:mnd
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

		DocOrder:      1,
		DocDeploys:    "[NixOS](https://nixos.org/manual/nixos/stable/) system",
		DocActivation: "`switch-to-configuration` (only type that supports bootstrap)",
	},
	FlakeOutputType("darwinConfigurations"): {
		BuildPath:      "system",
		ProfilePath:    "/nix/var/nix/profiles/system",
		SetProfile:     new(true),
		IsSystemLevel:  new(true),
		ActivationPath: "activate",

		DocOrder:      2,
		DocDeploys:    "[nix-darwin](https://github.com/nix-darwin/nix-darwin) (macOS)",
		DocActivation: "`activate` script",
	},
	FlakeOutputType("systemConfigs"): {
		BuildPath:      "",
		ProfilePath:    "/nix/var/nix/profiles/system-manager-profiles",
		SetProfile:     new(true),
		IsSystemLevel:  new(true),
		ActivationPath: "bin/activate",

		DocOrder:      3,
		DocDeploys:    "[system-manager](https://github.com/numtide/system-manager)",
		DocActivation: "`bin/activate`",
	},
	FlakeOutputType("homeConfigurations"): {
		BuildPath:      "activationPackage",
		ProfilePath:    "~/.local/state/nix/profiles/home-manager",
		IsSystemLevel:  new(false),
		ActivationPath: "activate",

		DocOrder:      4,
		DocDeploys:    "[home-manager](https://github.com/nix-community/home-manager)",
		DocActivation: "`activationPackage/activate`",
	},
	FlakeOutputType("nixOnDroidConfigurations"): {
		BuildPath:      "build.activationPackage",
		ProfilePath:    "~/.local/state/nix/profiles/nix-on-droid",
		IsSystemLevel:  new(false),
		ActivationPath: "activate",

		DocOrder:      5,
		DocDeploys:    "[Nix-on-Droid](https://github.com/nix-community/nix-on-droid)",
		DocActivation: "`activate` script",
	},
	FlakeOutputType("packages"): {
		BuildPath:            "",
		IsSystemLevel:        new(false),
		OmitTypeFromAttrPath: true,

		DocOrder:      6,
		DocDeploys:    "[Arbitrary packages](/guides/packages/)",
		DocActivation: "`nix profile add` (nix profile install under Lix)",
	},
	// User-defined output convention for nix-maid (https://github.com/viperML/nix-maid)
	// configuration bundles; activated via bin/activate in the closure.
	FlakeOutputType("maidConfigurations"): {
		IsSystemLevel:  new(false),
		ActivationPath: "bin/activate",

		DocOrder:      7,
		DocDeploys:    "[nix-maid](https://github.com/viperML/nix-maid)",
		DocActivation: "`bin/activate`",
	},
}

// OutputTypeTableRow is one row of the generated output-type tables.
type OutputTypeTableRow struct {
	Type       string
	Deploys    string
	Activation string
}

// OutputTypeTable returns the built-in output types for the generated README
// and docs tables, ordered by DocOrder with the type name as a tiebreaker.
func OutputTypeTable() []OutputTypeTableRow {
	rows := make([]OutputTypeTableRow, 0, len(presets))

	for typ, preset := range presets {
		rows = append(rows, OutputTypeTableRow{
			Type:       string(typ),
			Deploys:    preset.DocDeploys,
			Activation: preset.DocActivation,
		})
	}

	slices.SortFunc(rows, func(a, b OutputTypeTableRow) int {
		orderA := presets[FlakeOutputType(a.Type)].DocOrder
		orderB := presets[FlakeOutputType(b.Type)].DocOrder

		return cmp.Or(cmp.Compare(orderA, orderB), cmp.Compare(a.Type, b.Type))
	})

	return rows
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

// ActivationModes returns nixosConfigurations modes, feeding the bare
// activation_mode CLI completion.
func ActivationModes() []string {
	return presets[FlakeOutputType("nixosConfigurations")].ActivationModes
}
