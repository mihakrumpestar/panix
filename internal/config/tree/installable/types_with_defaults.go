package installable

// FlakeOutputType is the top-level flake output attribute name
// (e.g. "nixosConfigurations", "homeConfigurations", "packages").
// See: https://manual.determinate.systems/protocols/flake-schemas.html
type FlakeOutputType string

func (t FlakeOutputType) String() string {
	return string(t)
}

// AttributeName is the second-level attribute name within a flake output type
// (e.g. "server1" in "nixosConfigurations.server1").
// In Nix terminology, this is an "attribute name", the second element of
// the attrpath "nixosConfigurations.server1".
type AttributeName string

func (n AttributeName) String() string {
	return string(n)
}

// GetBuildPath returns the build path for this output type's preset.
// Returns empty string for root-level types (systemConfigs, packages)
// where the output itself is the derivation; no sub-attribute to append.
func (t FlakeOutputType) GetBuildPath() string {
	p, ok := presets[t]
	if !ok {
		return ""
	}

	return p.BuildPath
}

// GetProfilePath returns the nix profile path for this output type's preset.
// Returns empty string for types that don't use profile management (packages).
func (t FlakeOutputType) GetProfilePath() string {
	p, ok := presets[t]
	if !ok {
		return ""
	}

	return p.ProfilePath
}

// ShouldSetProfile returns true if nix-env --profile --set should be run
// before activation for this output type.
func (t FlakeOutputType) ShouldSetProfile() bool {
	p, ok := presets[t]
	if !ok {
		return false
	}

	return p.SetProfile
}

// ResolveFlakeInstallable constructs the full nix installable attrpath
// from the output type, attribute name, and build path.
// For root-level types (empty build path), returns just "type.name".
// For others, returns "type.name.buildPath".
func ResolveFlakeInstallable(outputType FlakeOutputType, attrName AttributeName) string {
	base := outputType.String() + "." + attrName.String()

	buildPath := outputType.GetBuildPath()
	if buildPath == "" {
		return base
	}

	return base + "." + buildPath
}

// CompositeKey creates a flat map key from output type and attribute name.
// Used for the flattened AtomicOrderedMap: "nixosConfigurations/server1".
func CompositeKey(outputType FlakeOutputType, attrName AttributeName) string {
	return outputType.String() + "/" + attrName.String()
}


