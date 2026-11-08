package activate

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/runtimevars"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/argv"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/mihakrumpestar/panix/pkg/structdefault"
	"github.com/pkg/errors"
)

type Handler struct {
	ActivationMode flags.ActivationMode
	NixFlavor      nixver.Flavor
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machine := fleetLeaf.Machine

	systemClosure := fleetLeaf.Installable.MetaBuild.Closure

	// Missing inspect data is unknown, not a fresh machine: the zero value
	// must never decide that the target is unbootstrapped.
	metaInspect := machine.MetaInspect.Load()

	inspected := metaInspect != nil && metaInspect.Inspected
	isBootstrapped := inspected && metaInspect.Bootstrapped

	// Run the NixOS install path if not bootstrapped, or force bootstrap is set
	shouldBootstrap := !isBootstrapped || machine.Bootstrap.ForceBootstrap

	if fleetLeaf.Installable.Preset.BootstrapsNixOS() && shouldBootstrap {
		err := validateNixOSInstallTarget(metaInspect)
		if err != nil {
			return err
		}

		return executeBootstrap(exc, machine, &fleetLeaf.Installable.Nix, systemClosure)
	}

	// Inspect validated the su -l precondition on its own connection, but
	// the bootstrap reboot can switch the active SSH since then: re-probe
	// rootness and re-validate before any wrapped command runs.
	if fleetLeaf.Installable.User != "" {
		err := phaseops.RefreshSuperuser(exc, machine)
		if err != nil {
			return err //nolint:wrapcheck // error is pre-annotated with its own context
		}

		err = phaseops.ValidateTargetUser(fleetLeaf.Installable, machine)
		if err != nil {
			return err //nolint:wrapcheck // error names installable, target user, and executing user
		}
	}

	return executeActivation(exc, h.ActivationMode, h.NixFlavor, fleetLeaf, systemClosure, &fleetLeaf.Installable.Nix)
}

// validateNixOSInstallTarget fails closed before the destructive
// nixos-install --root /mnt path: inspect must have succeeded, and a host that
// still requires kexec is not a NixOS installer yet (bootstrap was skipped).
func validateNixOSInstallTarget(metaInspect *machine.MetaInspect) error {
	if metaInspect == nil || !metaInspect.Inspected {
		// nixos-install --root /mnt followed by a reboot is destructive on
		// a live host; require a successful inspect before any install.
		return errors.New("inspect phase did not run; refusing to install to /mnt")
	}

	if metaInspect.RequiresKexec {
		// A non-NixOS host must first kexec into the NixOS installer (and
		// run disko); nixos-install cannot target the live host's /mnt.
		return errors.New("target is not a NixOS installer (requires kexec); refusing to install to /mnt")
	}

	return nil
}

func executeActivation(
	exc *executioner.Executioner,
	activationMode flags.ActivationMode,
	nixFlavor nixver.Flavor,
	fleetLeaf *fleet.FleetLeaf,
	closure string,
	nixCfg *nix.NixConfig,
) error {
	preset := &fleetLeaf.Installable.Preset

	mode := preset.ActivationDefaultMode
	if fleetLeaf.Installable.ActivationMode != "" {
		mode = fleetLeaf.Installable.ActivationMode
	}

	override := activationMode.Get(fleetLeaf.Installable.Type.String())
	if override != "" {
		mode = override
	}

	// Pre hooks gate the activation: a failure skips it and fails the
	// machine without rollback, the system is untouched.
	preErr := exc.ExecuteHooks(
		fleetLeaf.Machine.ActivationHooks.Pre,
		"pre activation hook",
		activationHookEnv(fleetLeaf, hookPhasePre, mode, closure),
	)
	if preErr != nil {
		return errors.Wrap(preErr, "pre activation hooks failed")
	}

	activationErr := phaseops.Activate(exc, fleetLeaf.Machine, *preset, closure, mode, fleetLeaf.Installable.User, nixCfg, nixFlavor)
	if activationErr == nil {
		return runPostActivationHooks(exc, fleetLeaf, mode, closure)
	}

	// No wrap here: the underlying command already prefixes its error with
	// statusIfFailed ("activation failed"); only the rollback outcome adds
	// context at this layer.
	originalErr := activationErr

	// Skip auto-rollback for non-mutating modes (nothing to restore), types
	// without a profile, and cancellations: rollback on a cancelled context
	// would fail instantly and bury the real error under rollback noise.
	if !fleetLeaf.Machine.AutoRollback ||
		slices.Contains(preset.NonMutatingModes, mode) ||
		preset.ProfilePath == "" ||
		errors.Is(activationErr, context.Canceled) {
		return originalErr
	}

	return autoRollbackToPreviousGeneration(exc, fleetLeaf, *preset, nixCfg, nixFlavor, originalErr)
}

// Hook phases exported to hooks as PANIX_HOOK_PHASE.
const (
	hookPhasePre  = "pre"
	hookPhasePost = "post"
)

// activationHookEnv builds the environment exported to every activation hook
// invocation. Values unavailable on this path (no inspect data, no profile)
// export as empty strings.
func activationHookEnv(fleetLeaf *fleet.FleetLeaf, hookPhase, mode, closure string) []string {
	var prevGeneration string

	mi := fleetLeaf.Machine.MetaInspect.Load()
	if mi != nil && mi.Generations != nil {
		prevGeneration = strconv.FormatUint(uint64(mi.Generations.Current), 10)
	}

	return []string{
		"PANIX_HOOK_PHASE=" + hookPhase,
		"PANIX_CLOSURE=" + closure,
		"PANIX_PROFILE_PATH=" + fleetLeaf.Installable.Preset.ProfilePath,
		"PANIX_PREV_GENERATION=" + prevGeneration,
		"PANIX_ACTIVATION_MODE=" + mode,
		"PANIX_INSTALLABLE=" + fleetLeaf.Installable.Name.String(),
		"PANIX_MACHINE=" + fleetLeaf.Machine.Name.String(),
	}
}

// runPostActivationHooks runs post activation hooks after a successful
// activation, skipped for non-mutating modes (nothing changed, nothing to
// follow up). A post-hook failure fails the phase without triggering auto
// rollback: the rollback path only runs for a failed activation, and rolling
// back a generation that activated fine would discard the deploy.
func runPostActivationHooks(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf, mode, closure string) error {
	if slices.Contains(fleetLeaf.Installable.Preset.NonMutatingModes, mode) {
		return nil
	}

	postErr := exc.ExecuteHooks(
		fleetLeaf.Machine.ActivationHooks.Post,
		"post activation hook",
		activationHookEnv(fleetLeaf, hookPhasePost, mode, closure),
	)
	if postErr != nil {
		return errors.Wrap(postErr, "post activation hooks failed")
	}

	return nil
}

func autoRollbackToPreviousGeneration(
	exc *executioner.Executioner,
	fleetLeaf *fleet.FleetLeaf,
	preset installable.Preset,
	nixCfg *nix.NixConfig,
	nixFlavor nixver.Flavor,
	originalErr error,
) error {
	metaInspect := fleetLeaf.Machine.MetaInspect.Load()
	if metaInspect == nil || metaInspect.Generations == nil || len(metaInspect.Generations.Available) == 0 {
		return originalErr
	}

	// metaInspect.Generations.Current is the pre-activation generation captured
	// during Inspect. It must NOT be refreshed after the failed activation,
	// otherwise the rollback would target the broken generation just activated.
	targetGen := metaInspect.Generations.Current

	// Announce the rollback in the command log: the following steps belong
	// to the pre-deploy generation, not a fresh deploy.
	logErr := exc.ExecFn(
		"auto rollback",
		"activation failed, rolling back to previous generation",
		"auto rollback failed",
		func(log *command.CommandLog) error {
			log.Output.Write(fmt.Appendf(nil, "activation failed: rolling back to generation %d", targetGen))

			return nil
		},
	)
	if logErr != nil {
		return errors.Wrapf(originalErr, "auto-rollback failed: %v", logErr)
	}

	closurePath, closureErr := phaseops.FindGenerationClosure(
		exc,
		fleetLeaf.Machine,
		preset,
		fleetLeaf.Installable.User,
		preset.ProfilePath,
		targetGen,
	)
	if closureErr != nil {
		return errors.Wrapf(originalErr, "auto-rollback failed to resolve generation: %v", closureErr)
	}

	rollbackErr := phaseops.Activate(
		exc,
		fleetLeaf.Machine,
		preset,
		closurePath,
		phaseops.RollbackActivationMode(preset),
		fleetLeaf.Installable.User,
		nixCfg,
		nixFlavor,
	)
	if rollbackErr != nil {
		return errors.Wrapf(originalErr, "auto-rollback failed: %v", rollbackErr)
	}

	return errors.Wrap(originalErr, "auto-rollback succeeded")
}

func executeBootstrap(exc *executioner.Executioner, machine *machine.Machine, nixCfg *nix.NixConfig, systemClosure string) error {
	// nixos-install writes to /mnt and installs the bootloader, so it must
	// run elevated, with the env(1) argv inside the sudo wrapper to survive
	// env_reset. The binary must be resolved before elevation.
	installArgv, installLabel, err := nixosInstallArgv(exc, nixCfg, systemClosure)
	if err != nil {
		return err
	}

	err = exc.Exec(
		"nixos-install",
		"installing NixOS",
		installLabel+" failed",
		append(machine.MaybeSudo(), installArgv...),
		executioner.Trim(),
	)
	if err != nil {
		return errors.Wrap(err, installLabel+" failed")
	}

	if len(machine.Bootstrap.PostBootstrapInstallHooks) > 0 {
		err = exc.ExecuteHooks(machine.Bootstrap.PostBootstrapInstallHooks, "post bootstrap install hook", nil)
		if err != nil {
			return errors.Wrap(err, "post bootstrap install hooks failed")
		}
	}

	if !machine.Bootstrap.DisableAutomaticReboot {
		err = verifyInstalledSystem(exc, machine, installedSystemProfile, systemClosure)
		if err != nil {
			return err
		}

		err = performReboot(exc, machine)
		if err != nil {
			return err
		}
	}

	if len(machine.Bootstrap.PostBootstrapProvisionedHooks) > 0 {
		err = exc.ExecuteHooks(machine.Bootstrap.PostBootstrapProvisionedHooks, "post bootstrap provisioned hook", nil)
		if err != nil {
			return errors.Wrap(err, "post bootstrap provisioned hooks failed")
		}
	}

	return nil
}

// installedSystemProfile is the profile nixos-install writes under --root /mnt
// (nixos-install runs `nix-env --profile <root>/nix/var/nix/profiles/system
// --set <system>`).
const installedSystemProfile = "/mnt/nix/var/nix/profiles/system"

// verifyInstalledSystem resolves the installed system profile and requires it to
// be the closure that was built.
//
// nixos-install is judged by exit status, so an overridden command that exits 0
// without installing would otherwise reach the irreversible reboot with no system
// on disk. readlink -f is not overridable, so this checks the install's effect
// rather than the tool's claim: without -f it returns only the generation link
// name (system-1-link), not the closure.
func verifyInstalledSystem(exc *executioner.Executioner, machine *machine.Machine, profilePath, systemClosure string) error {
	err := exc.Exec(
		"verify installed system",
		"verifying the installed system profile",
		"installed system profile does not resolve to the built closure",
		append(machine.MaybeSudo(), "readlink", "-f", profilePath),
		executioner.Trim(),
		executioner.OnSuccess(func(log *command.CommandLog) error {
			return requireInstalledClosure(profilePath, string(log.Output.LastLine()), systemClosure)
		}),
		executioner.OnDryRun(func() {}),
	)

	return err //nolint:wrapcheck // error is pre-annotated with statusIfFailed
}

// requireInstalledClosure compares the profile readback with the built closure,
// so an installer that exited 0 without installing fails before the reboot.
func requireInstalledClosure(profilePath, installed, systemClosure string) error {
	if installed != systemClosure {
		return errors.Errorf(
			"%s resolves to %q, not the built closure %q: refusing to reboot, the install did not take effect",
			profilePath, installed, systemClosure,
		)
	}

	return nil
}

// nixosInstallArgv builds the un-elevated nixos-install argv and the label the
// step reports: the configured command with its executable resolved on PATH, the
// env(1) prefix, the default flags (which carry the system closure and the
// target root), then the extra flags.
func nixosInstallArgv(
	exc *executioner.Executioner,
	nixCfg *nix.NixConfig,
	systemClosure string,
) ([]string, string, error) {
	command := nixCfg.NixosInstallCommand
	if len(command) == 0 {
		command = structdefault.List[nix.NixConfig]("NixosInstallCommand")
	}

	flags, err := runtimevars.Vars{runtimevars.Closure: systemClosure}.ExpandList(nixCfg.NixosInstallDefaultFlags)
	if err != nil {
		return nil, "", errors.Wrap(err, "nixos_install_default_flags")
	}

	// Without the closure the built-in installer installs whatever it finds in the
	// target's /etc/nixos instead of the system that was built, and the mistake
	// would only surface after the disk was written. Extra flags count too, so a
	// user can move the operand there. Any prefix element is checked, because a
	// wrapper such as ["env", "X=1", "nixos-install"] still runs the installer.
	if phaseops.CommandInvokes(command, "nixos-install") &&
		!slices.Contains(slices.Concat(flags, nixCfg.NixosInstallFlags), systemClosure) {
		return nil, "", errors.New(
			"nixos_install_default_flags dropped the built closure: the built-in nixos-install needs $PANIX_CLOSURE",
		)
	}

	resolved, err := resolveCommandPath(exc, command[0])
	if err != nil {
		return nil, "", err
	}

	installArgv := executioner.WithEnv(nixCfg.GetNixosInstallEnv(), argv.Argv(
		slices.Concat([]string{resolved}, command[1:]),
		nil,
		flags,
		nixCfg.NixosInstallFlags,
	))

	return installArgv, strings.Join(command, " "), nil
}

// isSSHDisconnect reports ssh's connection-lost exit status (255), which a
// command that takes the host down returns instead of a remote exit code.
func isSSHDisconnect(err error) bool {
	const sshConnectionLostStatus = 255

	var exitErr *exec.ExitError

	return errors.As(err, &exitErr) && exitErr.ExitCode() == sshConnectionLostStatus
}

// resolveCommandPath resolves commands un-elevated, because sudo's
// secure_path excludes the nix profile directories where installer
// commands live on non-NixOS hosts; absolute paths pass through unchanged.
func resolveCommandPath(exc *executioner.Executioner, cmdName string) (string, error) {
	if strings.HasPrefix(cmdName, "/") {
		return cmdName, nil
	}

	var resolved string

	err := exc.Exec(
		"resolve "+cmdName,
		fmt.Sprintf("resolving %s path", cmdName),
		"failed to resolve "+cmdName,
		[]string{"sh", "-c", "command -v -- " + shellquote.Quote(cmdName)},
		executioner.OnSuccess(func(log *command.CommandLog) error {
			resolved = strings.TrimSpace(log.Output.String())

			if resolved == "" {
				return errors.Errorf("%s not found on PATH on the target", cmdName)
			}

			return nil
		}),
		executioner.OnFailure(func(_ *command.CommandLog, err error) error {
			// command -v exits 1 when the command is missing.
			return errors.Errorf("%s not found on PATH on the target (command -v failed: %v)", cmdName, err)
		}),
		executioner.OnDryRun(func() {
			resolved = cmdName
		}),
	)
	if err != nil {
		return "", errors.Wrapf(err, "%s resolution failed", cmdName)
	}

	return resolved, nil
}

// Helpers

func performReboot(exc *executioner.Executioner, machineI *machine.Machine) error {
	// Elevated: reboot needs root (a no-op wrapper when the SSH user is
	// already root, e.g. inside the kexec installer).
	err := exc.Exec(
		"reboot",
		"rebooting",
		"reboot failed",
		append(machineI.MaybeSudo(), "reboot"),
		executioner.OnFailure(func(_ *command.CommandLog, err error) error {
			// The host can go down before ssh delivers reboot's exit status,
			// which surfaces as ssh's connection-lost status (255). That is the
			// expected outcome here, not a failed reboot. A local machine has no
			// ssh wrapper, so its status is authoritative.
			if !machineI.GetActiveSSH().IsLocal() && isSSHDisconnect(err) {
				return nil
			}

			return err
		}),
	)
	if err != nil {
		return errors.Wrap(err, "reboot failed")
	}

	if len(machineI.Bootstrap.PostBootstrapProvisionedHooks) == 0 {
		return nil
	}

	activeSSH := machineI.GetActiveSSH()

	err = executioner.WaitForDisconnect(exc, activeSSH, "waiting for machine to reboot")
	if err != nil {
		return errors.Wrap(err, "wait for disconnect failed")
	}

	machineI.State.Update(func(s *machine.State) { s.ActiveSSH = machine.SSHTypeRegular })
	activeSSH = machineI.GetActiveSSH()

	err = executioner.WaitForReconnect(exc, activeSSH, "waiting for machine to come back online", "machine did not reconnect after reboot")
	if err != nil {
		return errors.Wrap(err, "wait for reconnect failed")
	}

	return nil
}
