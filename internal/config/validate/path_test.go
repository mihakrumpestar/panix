package validate

import (
	"path/filepath"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	installablepkg "github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
)

// hardwarePathTestFleet builds a fleet holding one installable with the given
// machines, each named by its position and bootstrapping NixOS when asked.
func hardwarePathTestFleet(bootstraps bool, machines ...*machine.Machine) *fleet.Fleet {
	machinesMap := atomicorderedmap.New[string, *machine.Machine]()
	for index, machineI := range machines {
		machinesMap.Set("machine-"+string(rune('0'+index)), machineI)
	}

	inst := &installablepkg.Installable{}
	if bootstraps {
		inst.Preset = installablepkg.Preset{Bootstrap: installablepkg.BootstrapNixOS}
	}

	inst.Logs = logs.New()
	inst.Machines = machinesMap

	attrMap := atomicorderedmap.New[string, *installablepkg.Installable]()
	attrMap.Set("cfg0", inst)

	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Installables = atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]]()
	flakeObj.Installables.Set("nixosConfigurations", attrMap)

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()
	flakesMap.Set("flake0", flakeObj)

	return &fleet.Fleet{Flakes: flakesMap}
}

func newHardwarePathMachine(name, path string) *machine.Machine {
	machineI := &machine.Machine{}
	machineI.Xpath = xpath.New("flake0", "nixosConfigurations", "cfg0", name)
	machineI.HardwareConfig.Path = path

	return machineI
}

// TestValidateHardwareConfigPathUniqueness pins that two machines may not write
// the same local hardware config: generation would overwrite one with the other,
// and later runs would silently keep only the last machine's hardware. Paths are
// compared absolute, so two spellings of one file also collide.
func TestValidateHardwareConfigPathUniqueness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paths   []string
		wantErr bool
	}{
		{
			name:  "distinct paths pass",
			paths: []string{"/repo/host-a.nix", "/repo/host-b.nix"},
		},
		{
			name:  "one machine with a path passes",
			paths: []string{"/repo/host-a.nix"},
		},
		{
			name:  "machines without a path pass",
			paths: []string{"", ""},
		},
		{
			name:    "same path fails",
			paths:   []string{"/repo/hardware-configuration.nix", "/repo/hardware-configuration.nix"},
			wantErr: true,
		},
		{
			name:    "same file under a different spelling fails",
			paths:   []string{"/repo/host.nix", filepath.Join("/repo", ".", "host.nix")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			machines := make([]*machine.Machine, 0, len(tt.paths))
			for index, path := range tt.paths {
				machines = append(machines, newHardwarePathMachine("machine-"+string(rune('0'+index)), path))
			}

			errs := validateHardwareConfigPathUniqueness(hardwarePathTestFleet(true, machines...))

			if !tt.wantErr {
				assert.Empty(t, errs)

				return
			}

			assert.Len(t, errs, 1)
		})
	}
}

// Machines on an output type that never generates a hardware config cannot
// overwrite each other, so a shared path is not a conflict.
func TestValidateHardwareConfigPathUniqueness_IgnoresNonNixOSOutputType(t *testing.T) {
	t.Parallel()

	machines := []*machine.Machine{
		newHardwarePathMachine("machine-0", "/repo/hardware-configuration.nix"),
		newHardwarePathMachine("machine-1", "/repo/hardware-configuration.nix"),
	}

	errs := validateHardwareConfigPathUniqueness(hardwarePathTestFleet(false, machines...))

	assert.Empty(t, errs)
}
