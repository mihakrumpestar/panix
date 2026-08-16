package phaseops

import (
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/pkg/nixver"
)

// SetProfile runs `nix-env --profile <profilePath> --set <closure>`.
// Uses sudo if the machine's SSH user is not root.
func SetProfile(exc *executioner.Executioner, mach *machine.Machine, profilePath, closure string) error {
	return exc.Exec( //nolint:wrapcheck // error is pre-annotated with statusIfFailed
		"set profile",
		"setting profile: "+profilePath,
		"failed to set profile",
		append(mach.MaybeSudo(), "nix-env", "--profile", profilePath, "--set", closure),
		executioner.Trim(),
	)
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
	nixFlavor nixver.Flavor,
) error {
	err := maybeSetProfile(exc, mach, preset, closure, mode)
	if err != nil {
		return err
	}

	if preset.ActivationPath == "" {
		return activatePackage(exc, preset, closure, targetUser, nixCfg, nixFlavor)
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

	// Modes declared as profile-skipping by the preset (for NixOS: test and
	// dry-activate) evaluate the closure without committing to it, so the
	// profile keeps pointing at the current generation.
	if slices.Contains(preset.ProfileSkipModes, mode) {
		return nil
	}

	// No wrap: the executioner already prefixes errors with the command's statusIfFailed ("failed to set profile").
	return SetProfile(exc, mach, preset.ProfilePath, closure)
}

func activatePackage(
	exc *executioner.Executioner,
	preset installable.Preset,
	closure string,
	targetUser string,
	nixCfg *nix.NixConfig,
	nixFlavor nixver.Flavor,
) error {
	// Lix doesn't support `nix profile add` (Nix 2.30 renamed install to add,
	// but Lix never adopted the rename). Use `install` when Lix is detected;
	// keep `add` as default for Nix.
	profileSubcmd := profileSubcmdForFlavor(nixFlavor)

	args := slices.Concat(
		[]string{"nix"},
		nixCfg.GetExperimentalFeatures(),
		[]string{"profile", profileSubcmd},
		nixCfg.GetProfileAddDefaultFlags(),
		[]string{closure},
	)

	if !preset.IsSystemLevelValue() && targetUser != "" {
		args = asUser(targetUser, args)
	}

	return exc.Exec( //nolint:wrapcheck // error is pre-annotated with statusIfFailed
		"activate", "installing package", "package installation failed",
		args,
		executioner.Env(nixCfg.GetProfileAddEnv()),
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
	args := []string{}
	if preset.IsSystemLevelValue() {
		args = append(args, mach.MaybeSudo()...)
	}

	args = append(args, closure+"/"+preset.ActivationPath)
	if len(preset.ActivationModes) > 0 {
		args = append(args, mode)
	}

	if !preset.IsSystemLevelValue() && targetUser != "" {
		args = asUser(targetUser, args)
	}

	return exc.Exec( //nolint:wrapcheck // error is pre-annotated with statusIfFailed
		"activate", "activating", "activation failed",
		args,
		executioner.Trim(),
	)
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
// Quoting is applied minimally to the command itself: only args containing
// shell-unsafe characters are single-quoted. The command is then prefixed with
// an XDG_RUNTIME_DIR assignment (su -l does not set it) and always wrapped in
// an outer double-quote pair, since the prefix introduces whitespace and the
// -c argument must be a single shell word for SSH transport.
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

	// Step 2: su -l does not set XDG_RUNTIME_DIR (pam_systemd only sets it
	// for real login sessions). User-level tools such as systemd-tmpfiles
	// --user and sd-switch need it to locate the user D-Bus socket at
	// /run/user/<uid>/bus. Prepend the assignment; $(id -u) is expanded by
	// the login shell spawned by su, yielding the target user's UID.
	inner := xdgRuntimeDirPrefix + strings.Join(quoted, " ")

	// Step 3: wrap in double quotes so SSH's space-joining treats it as a
	// single argument to `su -c`. Characters special inside double quotes are
	// escaped to survive the remote shell's double-quote processing; this
	// turns the prefix's $ into \$ so the remote shell passes a literal
	// $(id -u) through to the login shell, which then expands it. Tilde
	// expansion does not happen inside double quotes, so ~ passes through to
	// the login shell where it IS expanded (because it's unquoted within the
	// command string).
	inner = escapeDoubleQuoteSpecials(inner)
	inner = `"` + inner + `"`

	return []string{"su", "-l", user, "-c", inner}
}

// shellSafeChars are characters that don't need shell quoting.
// Tilde (~) is included so paths like ~/.local/... are left unquoted,
// allowing the login shell to expand ~ to the target user's home.
// Equals (=) is included for --flag=value style arguments.
const shellSafeChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-./,:@+~="

// xdgRuntimeDirPrefix is prepended to user-level activation commands so the
// login shell sets XDG_RUNTIME_DIR, which su -l does not. $(id -u) resolves
// to the target user's UID inside that login shell.
const xdgRuntimeDirPrefix = `XDG_RUNTIME_DIR=/run/user/$(id -u) `

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

// profileSubcmdForFlavor returns the `nix profile` subcommand for the given
// nix implementation. Lix never adopted the Nix 2.30 rename of `install` to
// `add`, so it needs `install`. Nix supports both; `add` is the modern default.
func profileSubcmdForFlavor(flavor nixver.Flavor) string {
	if flavor == nixver.FlavorLix {
		return "install"
	}

	return "add"
}
