package config

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/phase"
)

type optionalPhases struct {
	Secrets   bool
	Bootstrap bool
}

func (c *Config) FilterOutUnusedPhases() {
	hasRequiredPhases := hasRequiredPhases(c.Fleet)

	c.Phases = slices.DeleteFunc(slices.Clone(c.Phases), func(p phase.Phase) bool {
		return (p == phase.Secrets && !hasRequiredPhases.Secrets) ||
			(p == phase.Bootstrap && !hasRequiredPhases.Bootstrap)
	})
}

func hasRequiredPhases(f *fleet.Fleet) optionalPhases {
	var has optionalPhases

	for _, fleetLeaf := range f.AllMachines() {
		mach := fleetLeaf.Machine
		if len(mach.Secrets) > 0 {
			has.Secrets = true
		}

		// A machine keeps the bootstrap phase only when its preset
		// bootstraps something. post_bootstrap_hooks run as part of a
		// bootstrap (after the Nix installer, or after disko on the NixOS
		// path) and never create one, so a machine that will not bootstrap
		// does not keep the phase.
		if fleetLeaf.Installable.Preset.IsBootstrappable() {
			if mach.Bootstrap.SSH.IsInitialized() || mach.Bootstrap.ForceBootstrap {
				has.Bootstrap = true
			}

			// The nix-install bootstrap mode installs Nix on targets that lack it,
			// over the regular SSH connection, without any bootstrap config. That
			// asymmetry with the NixOS bootstrap gate above is deliberate: kexec
			// and disko are destructive and demand explicit intent (bootstrap ssh
			// or force_bootstrap), while an automatic Nix install is additive and
			// idempotent. Opt out with bootstrap.disable_nix_install.
			if fleetLeaf.Installable.Preset.BootstrapsNix() && !mach.Bootstrap.DisableNixInstall {
				has.Bootstrap = true
			}
		}

		// Early return since we already have all possible optional phases
		if has.Secrets && has.Bootstrap {
			break
		}
	}

	return has
}
