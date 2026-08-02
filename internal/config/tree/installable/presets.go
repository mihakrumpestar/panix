package installable

// Preset defines the build path, activation mechanism, and profile management
// for a specific FlakeOutputType. Each known output type has a preset that
// auto-infers behavior, users don't need to specify these manually.
type Preset struct {
	// BuildPath is the attribute path appended to the installable to find
	// the buildable derivation. Empty means the output itself is the derivation.
	BuildPath string

	// ProfilePath is the nix profile path where the closure is set via
	// `nix-env --profile <path> --set`. Empty means no profile management.
	ProfilePath string

	// SetProfile controls whether nix-env --profile --set is run before activation.
	SetProfile bool

	// IsSystemLevel indicates whether this output type targets the system
	// (requires root) vs user-level (activates as the SSH user).
	IsSystemLevel bool

	// ActivationPath is the path within the closure to the activation script/binary.
	// e.g. "bin/switch-to-configuration" for nixosConfigurations, "activate" for home-manager.
	// For packages (no activation script), this is empty and activation uses nix profile install.
	ActivationPath string

	// SupportsModes indicates whether the activation command accepts a mode argument
	// (switch/boot/test/dry-activate). Only nixosConfigurations supports this.
	SupportsModes bool

	// IsBootstrappable indicates whether this output type supports bootstrap
	// (kexec, disko, nixos-install). Only nixosConfigurations supports this.
	IsBootstrappable bool
}

// presets maps each known FlakeOutputType to its Preset.
//
// Sources for activation mechanisms:
//   - nixosConfigurations: nixos-rebuild → switch-to-configuration
//   - darwinConfigurations: nix-darwin → activate script
//   - systemConfigs: system-manager → bin/activate
//   - homeConfigurations: home-manager → activationPackage/activate
//   - nixOnDroidConfigurations: nix-on-droid → activate script
//   - packages: nix profile install
var presets = map[FlakeOutputType]Preset{
	FlakeOutputType("nixosConfigurations"): {
		BuildPath:        "config.system.build.toplevel",
		ProfilePath:      "/nix/var/nix/profiles/system",
		SetProfile:       true,
		IsSystemLevel:    true,
		ActivationPath:   "bin/switch-to-configuration",
		SupportsModes:    true,
		IsBootstrappable: true,
	},
	FlakeOutputType("darwinConfigurations"): {
		BuildPath:      "system",
		ProfilePath:    "/nix/var/nix/profiles/system",
		SetProfile:     true,
		IsSystemLevel:  true,
		ActivationPath: "activate",
	},
	FlakeOutputType("systemConfigs"): {
		BuildPath:      "",
		ProfilePath:    "/nix/var/nix/profiles/system-manager-profiles",
		SetProfile:     true,
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
		// No ActivationPath: uses nix profile install
	},
}

// GetPreset returns the preset for the given output type.
// Returns ok=false for unknown types.
func GetPreset(t FlakeOutputType) (Preset, bool) {
	p, ok := presets[t]

	return p, ok
}

// knownOutputTypes is pre-computed.
var knownOutputTypes = func() []FlakeOutputType {
	types := make([]FlakeOutputType, 0, len(presets))
	for t := range presets {
		types = append(types, t)
	}

	return types
}()

// KnownOutputTypes returns all recognized installable types.
func KnownOutputTypes() []FlakeOutputType {
	return knownOutputTypes
}

// IsKnown returns true if the output type is recognized.
func (t FlakeOutputType) IsKnown() bool {
	_, ok := presets[t]

	return ok
}

// IsSystemLevel returns true if the output type targets the system (requires root).
func (t FlakeOutputType) IsSystemLevel() bool {
	p, ok := presets[t]
	if !ok {
		return false
	}

	return p.IsSystemLevel
}

// IsBootstrappable returns true if the output type supports bootstrap
// (kexec, disko, nixos-install). Currently only nixosConfigurations supports this.
func (t FlakeOutputType) IsBootstrappable() bool {
	p, ok := presets[t]
	if !ok {
		return false
	}

	return p.IsBootstrappable
}
