package phaseops

import (
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/runtimevars"
	"github.com/mihakrumpestar/panix/pkg/argv"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/pkg/errors"
)

// SetProfile sets the profile to closure, wrapped as the installable's target
// user via WrapAsTargetUser.
func SetProfile(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	profilePath string,
	closure string,
	targetUser string,
	nixCfg *nix.NixConfig,
) error {
	flags, err := runtimevars.Vars{
		runtimevars.Profile: profilePath,
		runtimevars.Closure: closure,
	}.ExpandList(nixCfg.NixEnvSetDefaultFlags)
	if err != nil {
		return errors.Wrap(err, "nix_env_set_default_flags")
	}

	// Without the profile, nix-env --set rewrites the default profile; without
	// the closure it sets the profile to nothing. Both succeed silently, so the
	// built-in command refuses the list instead.
	if CommandInvokes(nixCfg.NixEnvCommand, "nix-env") {
		err = RequireExpandedOperand("nix_env_set_default_flags", "$PANIX_PROFILE_PATH", flags, profilePath)
		if err != nil {
			return err
		}

		err = RequireExpandedOperand("nix_env_set_default_flags", "$PANIX_CLOSURE", flags, closure)
		if err != nil {
			return err
		}
	}

	cmd := argv.Argv(nixCfg.NixEnvCommand, nil, flags)

	return exc.Exec( //nolint:wrapcheck // error is pre-annotated with statusIfFailed
		"set profile",
		"setting profile: "+profilePath,
		"failed to set profile",
		WrapAsTargetUser(mach, preset, targetUser, cmd),
		executioner.Trim(),
	)
}

// Activate runs the activation for the given preset's output type, wrapping
// every command it issues as the installable's target user via
// WrapAsTargetUser.
func Activate(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	closure string,
	mode string,
	targetUser string,
	nixCfg *nix.NixConfig,
	nixFlavor nixver.Flavor,
) error {
	err := maybeSetProfile(exc, mach, preset, closure, mode, targetUser, nixCfg)
	if err != nil {
		return err
	}

	if preset.ActivationPath == "" {
		return activatePackage(exc, mach, preset, closure, targetUser, nixCfg, nixFlavor)
	}

	return activateScript(exc, mach, preset, closure, mode, targetUser)
}

func maybeSetProfile(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	closure string,
	mode string,
	targetUser string,
	nixCfg *nix.NixConfig,
) error {
	if preset.ProfilePath == "" || preset.SetProfile == nil || !*preset.SetProfile {
		return nil
	}

	// Modes declared as profile-skipping by the preset (for NixOS: test and
	// dry-activate) evaluate the closure without committing to it, so the
	// profile keeps pointing at the current generation.
	if slices.Contains(preset.ProfileSkipModes, mode) {
		return nil
	}

	// No wrap: the executioner prefixes errors with the command's statusIfFailed.
	return SetProfile(exc, mach, preset, preset.ProfilePath, closure, targetUser, nixCfg)
}

func activatePackage(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	closure string,
	targetUser string,
	nixCfg *nix.NixConfig,
	nixFlavor nixver.Flavor,
) error {
	// The default subcommand is flavor dependent, so ProfileAddCommand has no
	// struct-tag default: fall back to the flavor's built-in command.
	builtinCommand := len(nixCfg.ProfileAddCommand) == 0

	command := nixCfg.ProfileAddCommand
	if builtinCommand {
		command = []string{"nix", "profile", profileSubcmdForFlavor(nixFlavor)}
	}

	flags, err := runtimevars.Vars{runtimevars.Closure: closure}.ExpandList(nixCfg.ProfileAddDefaultFlags)
	if err != nil {
		return errors.Wrap(err, "profile_add_default_flags")
	}

	// Without the closure the built-in command installs the default package of
	// the working directory and exits 0, so it refuses the list instead.
	if builtinCommand {
		err = RequireExpandedOperand("profile_add_default_flags", "$PANIX_CLOSURE", flags, closure)
		if err != nil {
			return err
		}
	}

	args := argv.Argv(
		command,
		nixCfg.ExperimentalFeatures,
		flags,
	)

	// Wrap the full argv: system-level package types (custom presets) own
	// root-owned profiles, so MaybeSudo applies with no target user. The env
	// prefix stays inside the wrap so the variables survive sudo's env_reset.
	args = WrapAsTargetUser(mach, preset, targetUser, executioner.WithEnv(nixCfg.GetProfileAddEnv(), args))

	return exc.Exec( //nolint:wrapcheck // error is pre-annotated with statusIfFailed
		"activate", "installing package", "package installation failed",
		args,
		executioner.Trim(),
	)
}

func activateScript(
	exc *executioner.Executioner,
	mach *machine.Machine,
	preset installable.Preset,
	closure string,
	mode string,
	targetUser string,
) error {
	args := []string{closure + "/" + preset.ActivationPath}
	if len(preset.ActivationModes) > 0 {
		args = append(args, mode)
	}

	args = WrapAsTargetUser(mach, preset, targetUser, args)

	return exc.Exec( //nolint:wrapcheck // error is pre-annotated with statusIfFailed
		"activate", "activating", "activation failed",
		args,
		executioner.Trim(),
	)
}

// asUser is the quoting core of WrapAsTargetUser: it wraps a command as
// `su -l <user> -c "<command>"`. su instead of sudo because sudo may not be in
// PATH on NixOS (it is at /run/wrappers/bin/sudo); su is universally available.
//
// The -c string is a shell program: each argument is single-quoted
// (pkg/shellquote) to parse as one literal word, except ~, which stays bare for
// the login shell to expand to the target user's home. XDG_RUNTIME_DIR is
// prepended because su -l does not set it (pam_systemd does only for real login
// sessions) and tools such as systemd-tmpfiles --user and sd-switch need it for
// the user D-Bus socket at /run/user/<uid>/bus.
//
// The string is a single argv element, so su -c parses the identical string on
// local and remote machines.
func asUser(user string, command []string) []string {
	if user == "" {
		return command
	}

	quoted := make([]string, len(command))
	for i, arg := range command {
		quoted[i] = shellquote.QuoteWord(arg)
	}

	inner := xdgRuntimeDirPrefix + strings.Join(quoted, " ")

	return []string{"su", "-l", user, "-c", inner}
}

// NormalizedTargetUser maps a configured target user to the user the command
// runs as: system-level "root" becomes the no-user sudo path (sudo is how panix
// reaches root, and su -l root would prompt), user-level "root" stays a
// legitimate login-shell target.
func NormalizedTargetUser(preset installable.Preset, targetUser string) string {
	if preset.IsSystemLevelValue() && targetUser == "root" {
		return ""
	}

	return targetUser
}

// WrapAsTargetUser is the single public wrap for commands that run on a target
// machine: it applies the preset's privilege model and returns the argv to run.
//
//   - user-level: the SSH user, or the target user via `su -l <user> -c "<cmd>"`;
//     never elevated.
//   - system-level: elevated for whoever ends up running the command. The SSH
//     user gets MaybeSudo prefixed directly; a target user gets MaybeSudoFor
//     inside the su shell (su -l needs a root SSH user; a non-root target user
//     needs passwordless sudo; a root target user needs no inner elevation). The
//     XDG_RUNTIME_DIR prefix applies to the su login shell and may not survive
//     sudo's env_reset.
//
// Inspect validates the su -l precondition before any deploy phase runs.
func WrapAsTargetUser(mach *machine.Machine, preset installable.Preset, targetUser string, cmd []string) []string {
	targetUser = NormalizedTargetUser(preset, targetUser)

	switch {
	case !preset.IsSystemLevelValue():
		return asUser(targetUser, cmd)
	case targetUser == "":
		return append(mach.MaybeSudo(), cmd...)
	default:
		return asUser(targetUser, append(mach.MaybeSudoFor(targetUser), cmd...))
	}
}

// xdgRuntimeDirPrefix is the XDG_RUNTIME_DIR assignment asUser prepends;
// $(id -u) expands in the su login shell.
const xdgRuntimeDirPrefix = `XDG_RUNTIME_DIR=/run/user/$(id -u) `

// profileSubcmdForFlavor returns "install" for Lix, which never adopted the Nix
// 2.30 rename of `install` to `add`, and "add" for Nix (the modern default).
func profileSubcmdForFlavor(flavor nixver.Flavor) string {
	if flavor == nixver.FlavorLix {
		return "install"
	}

	return "add"
}
