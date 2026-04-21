package configuration

import "strings"

type FlakeOutput string

func (f FlakeOutput) Get() FlakeOutput {
	if f == "" {
		return "nixosConfigurations.<name>"
	}

	return f
}

func (f FlakeOutput) String() string {
	return string(f.Get())
}

type BuildPath string

func (b BuildPath) Get() BuildPath {
	if b == "" {
		return "config.system.build.toplevel"
	}

	return b
}

func (b BuildPath) String() string {
	return string(b.Get())
}

func ResolveFlakeInstallable(flakeOutput FlakeOutput, buildPath BuildPath, configName string) string {
	output := strings.ReplaceAll(flakeOutput.String(), "<name>", configName)

	return output + "." + buildPath.String()
}
