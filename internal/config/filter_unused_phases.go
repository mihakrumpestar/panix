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

		// Post-bootstrap hooks are user-declared and always run (see
		// runNixInstallBootstrap), so they keep the phase on their own. The
		// install and provisioned hook stages run only on the NixOS bootstrap
		// path, which is kept by the bootstrap config above.
		if len(mach.Bootstrap.PostBootstrapHooks) > 0 {
			has.Bootstrap = true
		}

		// Early return since we already have all possible optional phases
		if has.Secrets && has.Bootstrap {
			break
		}
	}

	return has
}
