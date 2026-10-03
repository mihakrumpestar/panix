package activate

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/logs/command"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops/guard"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/shellquote"
	"github.com/pkg/errors"
)

type Handler struct {
	ActivationMode flags.ActivationMode
	NixFlavor      nixver.Flavor
}

func (h Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machine := fleetLeaf.Machine

	systemClosure := fleetLeaf.Installable.MetaBuild.Closure

	isBootstrapped := false

	mi := machine.MetaInspect.Load()
	if mi != nil {
		isBootstrapped = mi.Bootstrapped
	}

	// Run bootstrap if not bootstrapped, or force bootstrap is set
	shouldBootstrap := !isBootstrapped || machine.Bootstrap.ForceBootstrap

	if fleetLeaf.Installable.Preset.IsBootstrappable && shouldBootstrap {
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

	mode := resolveActivationMode(h.ActivationMode, fleetLeaf.Installable)

	// Guarded tiers run the Activation Guard transaction (T3c); everything
	// else keeps the legacy direct activation with its in-process rollback.
	if shouldRouteToGuard(machine, fleetLeaf.Installable, mode) {
		return guard.ExecuteGuardedActivation(exc, machine, fleetLeaf.Installable.Preset,
			fleetLeaf.Installable.Type.String(), fleetLeaf.Installable.User, systemClosure, mode)
	}

	return executeActivation(exc, mode, h.NixFlavor, fleetLeaf, systemClosure, &fleetLeaf.Installable.Nix)
}

// resolveActivationMode applies the mode precedence (spec 16): installable
// override, preset default, CLI override. One source of truth for both the
// guarded and the legacy path.
func resolveActivationMode(cliMode flags.ActivationMode, installable *installable.Installable) string {
	mode := installable.Preset.ActivationDefaultMode
	if installable.ActivationMode != "" {
		mode = installable.ActivationMode
	}

	if override := cliMode.Get(installable.Type.String()); override != "" {
		mode = override
	}

	return mode
}

// shouldRouteToGuard resolves the effective guard eligibility (spec 2, T3c):
// the user tier (auto/magic = guarded), a supported tier class for the output
// type, a mutating mode, a profile to own, and a previous generation to
// protect. A first deploy has no rollback target: the guardian's precondition
// would fail it (FAILED_PRECONDITION), so it stays on the legacy path.
func shouldRouteToGuard(machineInstance *machine.Machine, installable *installable.Installable, mode string) bool {
	if !guard.ShouldGuard(
		guard.ConfirmationGate(machineInstance.Rollback.Get()),
		installable.Preset.GuardTierValue(),
		mode,
		installable.Preset.NonMutatingModes,
		installable.Preset.ProfilePath,
	) {
		return false
	}

	mi := machineInstance.MetaInspect.Load()

	return mi != nil && mi.Generations != nil && len(mi.Generations.Available) > 0
}

func executeActivation(
	exc *executioner.Executioner,
	mode string,
	nixFlavor nixver.Flavor,
	fleetLeaf *fleet.FleetLeaf,
	closure string,
	nixCfg *nix.NixConfig,
) error {
	preset := &fleetLeaf.Installable.Preset

	activationErr := phaseops.Activate(exc, fleetLeaf.Machine, *preset, closure, mode, fleetLeaf.Installable.User, nixCfg, nixFlavor)
	if activationErr == nil {
		return nil
	}

	// No wrap here: the underlying command already prefixes its error with
	// statusIfFailed ("activation failed"); only the rollback outcome adds
	// context at this layer.
	originalErr := activationErr

	// Skip rollback for non-guarded tiers that still reach this path
	// (tier-unsupported types, first deploys), non-mutating modes (nothing to
	// restore), types without a profile, and cancellations: rollback on a
	// cancelled context would fail instantly and bury the real error under
	// rollback noise.
	if !fleetLeaf.Machine.Rollback.IsGuarded() ||
		slices.Contains(preset.NonMutatingModes, mode) ||
		preset.ProfilePath == "" ||
		errors.Is(activationErr, context.Canceled) {
		return originalErr
	}

	return autoRollbackToPreviousGeneration(exc, fleetLeaf, *preset, nixCfg, nixFlavor, originalErr)
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
	// run elevated, with the env(1) argv inside the sudo prefix to survive
	// env_reset. The binary must be resolved before elevation.
	nixosInstall, err := resolveCommandPath(exc, "nixos-install")
	if err != nil {
		return err
	}

	err = exc.Exec(
		"nixos-install",
		"installing NixOS",
		"nixos-install failed",
		append(machine.MaybeSudo(), phaseops.WithEnv(nixCfg.GetNixosInstallEnv(), slices.Concat(
			[]string{nixosInstall},
			nixCfg.GetNixosInstallDefaultFlags(),
			[]string{"--system", systemClosure, "--root", "/mnt"},
			nixCfg.NixosInstallFlags,
		))...),
		executioner.Trim(),
	)
	if err != nil {
		return errors.Wrap(err, "nixos-install failed")
	}

	if len(machine.Bootstrap.PostBootstrapInstallHooks) > 0 {
		err = exc.ExecuteHooks(machine.Bootstrap.PostBootstrapInstallHooks, "post bootstrap install hook")
		if err != nil {
			return errors.Wrap(err, "post bootstrap install hooks failed")
		}
	}

	if !machine.Bootstrap.DisableAutomaticReboot {
		err = performReboot(exc, machine)
		if err != nil {
			return err
		}
	}

	if len(machine.Bootstrap.PostBootstrapProvisionedHooks) > 0 {
		err = exc.ExecuteHooks(machine.Bootstrap.PostBootstrapProvisionedHooks, "post bootstrap provisioned hook")
		if err != nil {
			return errors.Wrap(err, "post bootstrap provisioned hooks failed")
		}
	}

	return nil
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
	// Elevated: reboot needs root (a no-op prefix when the SSH user is
	// already root, e.g. inside the kexec installer).
	err := exc.Exec(
		"reboot",
		"rebooting",
		"reboot failed",
		append(machineI.MaybeSudo(), "reboot"),
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
