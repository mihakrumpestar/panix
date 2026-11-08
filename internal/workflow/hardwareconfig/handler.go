// Package hardwareconfig owns the hardware-config phase: generating a machine's
// hardware config on the target and writing it to the local path in the flake.
package hardwareconfig

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var (
	// ErrNoHardwareConfigTargets reports a configuration with no machine that
	// declares a hardware config on a NixOS-bootstrapping output type.
	ErrNoHardwareConfigTargets = errors.New(
		"no machine sets hardware_config.path on a NixOS-bootstrapping output type " +
			"(after tag and disabled filtering), so there is no hardware config to generate",
	)

	// ErrPhaseSkipped reports a run that skips the phase this command exists for.
	ErrPhaseSkipped = errors.New(
		"hardware-config cannot skip its own phase (remove \"hardware-config\" from skip_phases)",
	)

	// ErrKexecRequiredGeneration reports a machine that still needs kexec: the
	// generator exists only in the installer, so a deploy has to produce the file.
	ErrKexecRequiredGeneration = errors.New(
		"the machine still needs kexec, so the generator is unavailable; " +
			"run a deploy, which generates the config right after kexec",
	)
)

// Handler generates the hardware config of a machine that was installed before
// the run started. It runs right after Inspect, so transport and elevation match
// the rest of the workflow.
type Handler struct{}

// ShouldSkip skips machines with nothing to generate for: no configured path, or
// an output type that does not bootstrap NixOS. Whether this run should generate
// for the machine depends on the run mode, so RunPhase decides that.
func (Handler) ShouldSkip(fleetLeaf *fleet.FleetLeaf) bool {
	machineI := fleetLeaf.Machine

	reason := ""

	switch {
	case machineI.HardwareConfig.Path == "":
		reason = "hardware_config.path is not set"
	case !fleetLeaf.Installable.Preset.BootstrapsNixOS():
		reason = "the output type does not bootstrap NixOS"
	}

	if reason == "" {
		return false
	}

	log.Info().Str("xpath", machineI.Xpath.String()).Msg("skipping hardware config: " + reason)

	return true
}

// RunPhase generates the hardware config and writes it to the local path.
func (Handler) RunPhase(exc *executioner.Executioner, fleetLeaf *fleet.FleetLeaf) error {
	machineI := fleetLeaf.Machine

	switch decide(machineI, exc.DryRun()) {
	case actionGenerate:
	case actionSkipInstallerMachine:
		log.Info().Str("xpath", machineI.Xpath.String()).
			Msg("skipping hardware config: Inspect already generated it for the installer machine")

		return nil
	case actionKexecRequired:
		return ErrKexecRequiredGeneration
	}

	return phaseops.GenerateHardwareConfig(exc, machineI) //nolint:wrapcheck // error is pre-annotated with its own context
}

// Prepare rejects a run where the phase cannot do its job: the phase is skipped,
// a target generator would write instead of print, or nothing declares a config
// it could generate. It runs before any machine is contacted, so the rejection
// names the configuration mistake.
func Prepare(conf *config.Config) error {
	if slices.Contains(conf.Flags.SkipPhases, phase.HardwareConfig) {
		return ErrPhaseSkipped
	}

	found := false

	for _, fleetLeaf := range conf.Fleet.AllMachines() {
		machineI := fleetLeaf.Machine
		if machineI.HardwareConfig.Path == "" || !fleetLeaf.Installable.Preset.BootstrapsNixOS() {
			continue
		}

		if phaseops.IsWriteModeGenerator(machineI.HardwareConfig.Generate) {
			return errors.Wrapf(phaseops.ErrWriteModeGenerator, "%s: hardware_config.generate", machineI.Xpath.String())
		}

		found = true
	}

	if !found {
		return ErrNoHardwareConfigTargets
	}

	return nil
}

// phaseAction is what the phase does for one machine.
type phaseAction int

const (
	actionGenerate phaseAction = iota
	actionSkipInstallerMachine
	actionKexecRequired
)

// decide picks the action for one machine. A dry run executes no commands, so
// Inspect recorded placeholder installer state; the preview shows what a real run
// would do. A real run leaves the installer machine alone, because Inspect
// already generated its file, and reports a pre-kexec machine, which only a
// deploy can serve.
func decide(machineI *machine.Machine, dryRun bool) phaseAction {
	if dryRun {
		return actionGenerate
	}

	switch {
	case requiresKexec(machineI):
		return actionKexecRequired
	case !isBootstrapped(machineI):
		return actionSkipInstallerMachine
	default:
		return actionGenerate
	}
}

// requiresKexec reports a machine that has not reached the installer yet, so the
// generator does not exist on it.
func requiresKexec(machineI *machine.Machine) bool {
	metaInspect := machineI.MetaInspect.Load()

	return metaInspect != nil && metaInspect.RequiresKexec
}

func isBootstrapped(machineI *machine.Machine) bool {
	metaInspect := machineI.MetaInspect.Load()

	return metaInspect != nil && metaInspect.Bootstrapped
}
