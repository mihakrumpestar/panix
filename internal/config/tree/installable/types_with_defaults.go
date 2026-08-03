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

// ResolveFlakeInstallable constructs the full nix installable attrpath
// from the output type, attribute name, and build path.
// For root-level types (empty build path), returns just "type.name".
// For others, returns "type.name.buildPath".
func ResolveFlakeInstallable(outputType FlakeOutputType, attrName AttributeName, buildPath string) string {
	base := outputType.String() + "." + attrName.String()

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


