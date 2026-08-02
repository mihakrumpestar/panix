package phaseops

import (
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/pkg/errors"
)

// SetProfile runs `nix-env --profile <profilePath> --set <closure>`.
// Uses sudo if the machine's SSH user is not root.
func SetProfile(exc *executioner.Executioner, mach *machine.Machine, profilePath, closure string) error {
	err := exc.Exec(
		"set profile",
		"setting profile: "+profilePath,
		"failed to set profile",
		append(mach.MaybeSudo(), "nix-env", "--profile", profilePath, "--set", closure),
		executioner.Trim(),
	)

	return errors.Wrap(err, "failed to set profile")
}

// Activate runs the activation for the given preset's output type.
// For profiled types: optionally sets the profile, then runs the activation script.
// For packages: runs nix profile install.
func Activate(exc *executioner.Executioner, mach *machine.Machine, preset installable.Preset, closure string, mode attributes.ActivationMode) error {
	if preset.ProfilePath != "" && preset.SetProfile {
		if !preset.SupportsModes || (mode != attributes.ActivationModeTest && mode != attributes.ActivationModeDryActivate) {
			err := SetProfile(exc, mach, preset.ProfilePath, closure)
			if err != nil {
				return errors.Wrap(err, "failed to set profile")
			}
		}
	}

	if preset.ActivationPath == "" {
		// packages type: nix profile install
		err := exc.Exec(
			"activate", "installing package", "package installation failed",
			[]string{"nix", "--extra-experimental-features", "nix-command flakes", "profile", "install", closure},
			executioner.Trim(),
		)

		return errors.Wrap(err, "failed to install package")
	}

	args := []string{}
	if preset.IsSystemLevel {
		args = append(args, mach.MaybeSudo()...)
	}

	args = append(args, closure+"/"+preset.ActivationPath)
	if preset.SupportsModes {
		args = append(args, string(mode))
	}

	err := exc.Exec(
		"activate", "activating", "activation failed",
		args,
		executioner.Trim(),
	)

	return errors.Wrap(err, "failed to activate")
}
