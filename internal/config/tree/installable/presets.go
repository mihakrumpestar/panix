package installable

// Preset defines the build path, activation mechanism, and profile management
// for a specific FlakeOutputType. Each known output type has a preset that
// auto-infers behavior, users don't need to specify these manually.
// All fields are user-configurable via YAML; empty values use the type default.
type Preset struct {
	BuildPath             string   `yaml:"build_path,omitempty" json:"build_path,omitempty" desc:"Build path within the flake output"`
	ProfilePath           string   `yaml:"profile_path,omitempty" json:"profile_path,omitempty" desc:"Nix profile path"`
	ActivationPath        string   `yaml:"activation_path,omitempty" json:"activation_path,omitempty" desc:"Activation script path in the closure"`
	SetProfile            *bool    `yaml:"set_profile,omitempty" json:"set_profile,omitempty" desc:"Run nix-env --profile --set before activation"`
	IsSystemLevel         bool     `yaml:"is_system_level,omitempty" json:"is_system_level,omitempty" desc:"System-level (root) vs user-level"`
	IsBootstrappable      bool     `yaml:"is_bootstrappable,omitempty" json:"is_bootstrappable,omitempty" desc:"Supports bootstrap"`
	ActivationModes       []string `yaml:"activation_supported_modes,omitempty" json:"activation_supported_modes,omitempty" desc:"Activation modes"`
	ActivationDefaultMode string   `yaml:"activation_default_mode,omitempty" json:"activation_default_mode,omitempty" desc:"Default activation mode"`
}

// presets maps each known FlakeOutputType to its Preset.
var presets = map[FlakeOutputType]Preset{
	FlakeOutputType("nixosConfigurations"): {
		BuildPath:             "config.system.build.toplevel",
		ProfilePath:           "/nix/var/nix/profiles/system",
		SetProfile:            new(true),
		IsSystemLevel:         true,
		ActivationPath:        "bin/switch-to-configuration",
		ActivationModes:       []string{"switch", "boot", "test", "dry-activate"},
		ActivationDefaultMode: "switch",
		IsBootstrappable:      true,
	},
	FlakeOutputType("darwinConfigurations"): {
		BuildPath:      "system",
		ProfilePath:    "/nix/var/nix/profiles/system",
		SetProfile:     new(true),
		IsSystemLevel:  true,
		ActivationPath: "activate",
	},
	FlakeOutputType("systemConfigs"): {
		BuildPath:      "",
		ProfilePath:    "/nix/var/nix/profiles/system-manager-profiles",
		SetProfile:     new(true),
		IsSystemLevel:  true,
		ActivationPath: "bin/activate",
	},
	FlakeOutputType("homeConfigurations"): {
		BuildPath:      "activationPackage",
		ProfilePath:    "~/.local/state/nix/profiles/home-manager",
		IsSystemLevel:  false,
		ActivationPath: "activate",
	},
	FlakeOutputType("nixOnDroidConfigurations"): {
		BuildPath:      "build.activationPackage",
		ProfilePath:    "~/.local/state/nix/profiles/nix-on-droid",
		IsSystemLevel:  false,
		ActivationPath: "activate",
	},
	FlakeOutputType("packages"): {
		BuildPath:     "",
		IsSystemLevel: false,
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
