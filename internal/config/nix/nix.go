package nix

//nolint:lll
type NixConfig struct {
	BuildMode         BuildMode `yaml:"build_mode" json:"build_mode" desc:"Build mode: local or remote. In local mode, build runs on the local machine and the closure is copied to targets. In remote mode, build runs on the first target machine via --store ssh-ng://<target>. For single-machine configurations, transfer is skipped (closure is already on target). For multi-machine configurations, the closure is copied from the first machine to the rest." default:"local" validate:"omitempty,oneof=local remote"`
	ExtraFlags        []string  `yaml:"extra_flags" json:"extra_flags,omitempty" desc:"Extra flags applied to both 'nix build' and 'nix copy'"`
	BuildFlags        []string  `yaml:"build_flags" json:"build_flags,omitempty" desc:"Extra flags for 'nix build' command (e.g. '--max-jobs', '4')"`
	CopyFlags         []string  `yaml:"copy_flags" json:"copy_flags,omitempty" desc:"Extra flags for 'nix copy' command (e.g. '--compress')"`
	NixosInstallFlags []string  `yaml:"nixos_install_flags" json:"nixos_install_flags,omitempty" desc:"Extra flags for 'nixos-install' command (e.g. '--no-bootloader')"`
}

// Init sets defaults for zero-value fields.
func (c *NixConfig) Init() {
	if c.BuildMode == "" {
		c.BuildMode = BuildModeLocal
	}
}

// BuildMode determines how nix builds are executed.
type BuildMode string

const (
	// BuildModeLocal:
	// Build runs on the local machine. Closure is copied to the target via nix copy.
	BuildModeLocal BuildMode = "local"

	// BuildModeRemote:
	// Build runs on the first target machine via --store ssh-ng://<target>.
	// - Single-machine configuration: transfer is skipped (closure is already on target).
	// - Multi-machine configuration: closure is copied from the first machine to the rest
	//   via nix copy --from <first-machine> --to <other-machine>.
	BuildModeRemote BuildMode = "remote"
)

func (m BuildMode) String() string {
	return string(m)
}
