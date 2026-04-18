package configuration

type FlakeOutput string

func (f FlakeOutput) Get() FlakeOutput {
	if f == "" {
		return "nixosConfigurations.<name>.config.system.build.toplevel"
	}

	return f
}

func (f FlakeOutput) String() string {
	return string(f.Get())
}
