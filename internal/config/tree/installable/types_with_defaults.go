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

// NixOSSystemBuildPath is the attrpath suffix of the built system closure
// under a NixOS system output: the nixosConfigurations preset BuildPath, and
// nixos-install installs the system closure built at this path.
const NixOSSystemBuildPath = "config.system.build.toplevel"

// NixOSDiskoScriptPath is the attrpath suffix of the disko script under a
// NixOS system output: bootstrap disko resolves the disko script next to the
// system closure.
const NixOSDiskoScriptPath = "config.system.build.diskoScript"

// ResolveFlakeAttrBase constructs the base flake attrpath (output type and
// attribute name, without any build path suffix).
//
// The top-level attribute has this precedence:
//  1. preset.OutputTypeAttr when set, producing "outputTypeAttr.name".
//     This lets the user point the config key (e.g. nixosConfigurations) at a
//     differently-named flake output attribute (e.g. nixosConf).
//  2. For types where OmitTypeFromAttrPath is true (e.g. "packages"), nix
//     auto-resolves bare attribute names under "packages.<system>.<name>",
//     so the type prefix must be omitted. In that case returns just
//     "name".
//  3. Otherwise "type.name" (e.g. "nixosConfigurations.server1").
//
// An explicit output_type_attr takes precedence over omit_type_from_attr_path.
func ResolveFlakeAttrBase(outputType FlakeOutputType, attrName AttributeName, preset Preset) string {
	switch {
	case preset.OutputTypeAttr != "":
		return preset.OutputTypeAttr + "." + attrName.String()
	case preset.OmitTypeFromAttrPath:
		return attrName.String()
	default:
		return outputType.String() + "." + attrName.String()
	}
}

// ResolveFlakeInstallable constructs the full nix installable attrpath from
// the output type, attribute name, and preset: the base attrpath
// (ResolveFlakeAttrBase) with the preset's build path appended as
// "base[.buildPath]".
func ResolveFlakeInstallable(outputType FlakeOutputType, attrName AttributeName, preset Preset) string {
	base := ResolveFlakeAttrBase(outputType, attrName, preset)

	if preset.BuildPath == "" {
		return base
	}

	return base + "." + preset.BuildPath
}

// CompositeKey creates a flat map key from output type and attribute name.
// Used for the flattened AtomicOrderedMap: "nixosConfigurations/server1".
func CompositeKey(outputType FlakeOutputType, attrName AttributeName) string {
	return outputType.String() + "/" + attrName.String()
}
