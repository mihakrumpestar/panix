package phaseops

import (
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/nix"
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
// For system-level types: uses MaybeSudo() (sudo if SSH user isn't root).
// For user-level types with a target user: wraps activation in `su -l <user> -c`.
// For user-level types without a target user: runs as the SSH user directly.
// For packages: runs nix profile install.
func Activate(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	closure string,
	mode string,
	targetUser string,
	nixCfg *nix.NixConfig,
) error {
	err := maybeSetProfile(exc, mach, preset, closure, mode)
	if err != nil {
		return err
	}

	if preset.ActivationPath == "" {
		return activatePackage(exc, mach, preset, closure, targetUser, nixCfg)
	}

	return activateScript(exc, mach, preset, closure, mode, targetUser)
}

func maybeSetProfile(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	closure string,
	mode string,
) error {
	if preset.ProfilePath == "" || preset.SetProfile == nil || !*preset.SetProfile {
		return nil
	}

	if supportsMode(preset.ActivationModes, mode) && (mode == "test" || mode == "dry-activate") {
		return nil
	}

	return errors.Wrap(SetProfile(exc, mach, preset.ProfilePath, closure), "failed to set profile")
}

func activatePackage(
	exc *executioner.Executioner,
	_ *machine.Machine,
	preset installable.Preset,
	closure string,
	targetUser string,
	nixCfg *nix.NixConfig,
) error {
	args := slices.Concat(
		[]string{"nix"},
		nixCfg.GetExperimentalFeatures(),
		[]string{"profile", "install"},
		nixCfg.GetProfileInstallDefaultFlags(),
		[]string{closure},
	)

	if !preset.IsSystemLevel && targetUser != "" {
		args = asUser(targetUser, args)
	}

	err := exc.Exec(
		"activate", "installing package", "package installation failed",
		args,
		executioner.Env(nixCfg.GetProfileInstallEnv()),
		executioner.Trim(),
	)

	return errors.Wrap(err, "failed to install package")
}

func activateScript(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	closure string,
	mode string,
	targetUser string,
) error {
	args := []string{}
	if preset.IsSystemLevel {
		args = append(args, mach.MaybeSudo()...)
	}

	args = append(args, closure+"/"+preset.ActivationPath)
	if len(preset.ActivationModes) > 0 {
		args = append(args, mode)
	}

	if !preset.IsSystemLevel && targetUser != "" {
		args = asUser(targetUser, args)
	}

	err := exc.Exec(
		"activate", "activating", "activation failed",
		args,
		executioner.Trim(),
	)

	return errors.Wrap(err, "failed to activate")
}

// AsUser wraps a command to run as a different user via `su -l <user> -c '<command>'`.
// Uses su instead of sudo because sudo may not be in PATH on NixOS
// (it's at /run/wrappers/bin/sudo). su is universally available.
func AsUser(user string, command []string) []string {
	if user == "" {
		return command
	}

	cmdStr := strings.Join(command, " ")
	// Escape single quotes for shell safety
	cmdStr = strings.ReplaceAll(cmdStr, "'", `'\''`)

	return []string{"su", "-l", user, "-c", "'" + cmdStr + "'"}
}

func asUser(user string, command []string) []string {
	return AsUser(user, command)
}

// supportsMode checks if the given mode is in the supported modes list.
func supportsMode(supported []string, mode string) bool {
	return slices.Contains(supported, mode)
}
