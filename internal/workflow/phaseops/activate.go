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
// For packages: runs nix profile add.
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
		[]string{"profile", "add"},
		nixCfg.GetProfileAddDefaultFlags(),
		[]string{closure},
	)

	if !preset.IsSystemLevel && targetUser != "" {
		args = asUser(targetUser, args)
	}

	err := exc.Exec(
		"activate", "installing package", "package installation failed",
		args,
		executioner.Env(nixCfg.GetProfileAddEnv()),
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

// AsUser wraps a command to run as a different user via `su -l <user> -c "<command>"`.
// Uses su instead of sudo because sudo may not be in PATH on NixOS
// (it's at /run/wrappers/bin/sudo). su is universally available.
//
// The command must survive two layers of shell parsing:
//  1. SSH joins all args after the hostname with spaces and sends them to the
//     remote shell. The -c argument must be a single shell word so it isn't
//     split by the remote shell.
//  2. `su -c` runs the command string through a login shell, which re-parses
//     it. Arguments containing spaces (e.g. "nix-command flakes") must stay
//     together, and tilde (~) must remain unquoted for home directory expansion.
//
// Quoting is applied minimally: only args containing shell-unsafe characters
// are single-quoted, and the outer double-quote wrapper is only added when the
// joined string contains whitespace (i.e. multi-word commands).
//
// Note: This function is designed for the SSH execution path. When panix
// dispatches commands via SSH, the remote shell consumes the outer double
// quotes before `su -c` runs. For local (non-SSH) execution, the quoting
// would need to be different since exec.Command uses argv directly.
func AsUser(user string, command []string) []string {
	if user == "" {
		return command
	}

	// Step 1: shell-quote each arg individually. Only args with unsafe
	// characters (spaces, quotes, $, etc.) get single-quoted; safe args
	// (alphanumerics, paths, flags, ~) pass through unquoted.
	// Tilde (~) is left unquoted so the login shell can expand it to the
	// target user's home directory (e.g. ~/.local/state/nix/profiles/...).
	quoted := make([]string, len(command))
	for i, arg := range command {
		quoted[i] = shellQuote(arg)
	}

	inner := strings.Join(quoted, " ")

	// Step 2: wrap in double quotes if the string contains whitespace, so
	// SSH's space-joining treats it as a single argument to `su -c`.
	// Characters special inside double quotes are escaped to survive the
	// remote shell's double-quote processing. Tilde expansion does not
	// happen inside double quotes, so ~ passes through to the login shell
	// where it IS expanded (because it's unquoted within the command string).
	if strings.ContainsAny(inner, " \t\n") {
		inner = escapeDoubleQuoteSpecials(inner)
		inner = `"` + inner + `"`
	}

	return []string{"su", "-l", user, "-c", inner}
}

// shellSafeChars are characters that don't need shell quoting.
// Tilde (~) is included so paths like ~/.local/... are left unquoted,
// allowing the login shell to expand ~ to the target user's home.
// Equals (=) is included for --flag=value style arguments.
const shellSafeChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-./,:@+~="

// shellQuote wraps a string in single quotes if it contains any character
// that is not shell-safe. Empty strings are also quoted.
func shellQuote(str string) string {
	if str == "" {
		return "''"
	}

	if isShellSafe(str) {
		return str
	}

	return "'" + strings.ReplaceAll(str, "'", `'\''`) + "'"
}

func isShellSafe(str string) bool {
	for i := range len(str) {
		if !strings.ContainsRune(shellSafeChars, rune(str[i])) {
			return false
		}
	}

	return true
}

// escapeDoubleQuoteSpecials escapes characters that are special inside double
// quotes: backslash, double-quote, dollar, and backtick. Backslash is escaped
// first so we don't double-escape backslashes added by subsequent replacements.
func escapeDoubleQuoteSpecials(str string) string {
	str = strings.ReplaceAll(str, `\`, `\\`)
	str = strings.ReplaceAll(str, `"`, `\"`)
	str = strings.ReplaceAll(str, `$`, `\$`)
	str = strings.ReplaceAll(str, "`", "\\`")

	return str
}

func asUser(user string, command []string) []string {
	return AsUser(user, command)
}

// supportsMode checks if the given mode is in the supported modes list.
func supportsMode(supported []string, mode string) bool {
	return slices.Contains(supported, mode)
}
