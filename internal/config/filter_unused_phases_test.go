package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/phase"
	"github.com/mihakrumpestar/panix/internal/testutil"
)

//nolint:funlen
func TestHasRequiredPhases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		buildFleet    func() *fleet.Fleet
		wantSecrets   bool
		wantBootstrap bool
	}{
		{
			"no secrets no bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.Machine(), fk.Machine())))
			},
			false, false,
		},
		{
			"machine with secrets",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.Machine(), fk.MachineWithSecrets(2))))
			},
			true, false,
		},
		{
			"machine with bootstrap ssh",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.Machine(), fk.MachineWithBootstrapSSH())))
			},
			false, true,
		},
		{
			"machine with force bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.MachineWithForceBootstrap())))
			},
			false, true,
		},
		{
			"both secrets and bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.MachineWithSecrets(1), fk.MachineWithBootstrapSSH())))
			},
			true, true,
		},
		{
			"secrets in different flake",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(
					fk.Flake(fk.Installable(fk.MachineWithSecrets(1))),
					fk.Flake(fk.Installable(fk.MachineWithBootstrapSSH())),
				)
			},
			true, true,
		},
		{
			"empty fleet",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable()))
			},
			false, false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			result := hasRequiredPhases(test.buildFleet())
			assertion.Equal(test.wantSecrets, result.Secrets)
			assertion.Equal(test.wantBootstrap, result.Bootstrap)
		})
	}
}

//nolint:funlen
func TestFilterOutUnusedPhases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		buildFleet  func() *fleet.Fleet
		inputPhases []phase.Phase
		wantPhases  []phase.Phase
	}{
		{
			"removes secrets when no secrets",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
		{
			"removes bootstrap when no bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
		{
			"removes both unused phases",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
		{
			"keeps secrets when machine has secrets",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.MachineWithSecrets(1))))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Secrets, phase.Activate},
		},
		{
			"keeps bootstrap when machine has bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.MachineWithBootstrapSSH())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Activate},
		},
		{
			"keeps all phases when both needed",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.MachineWithSecrets(1), fk.MachineWithBootstrapSSH())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
		},
		{
			"does not remove phases not in list",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Installable(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			c := &Config{
				Fleet:  test.buildFleet(),
				Phases: test.inputPhases,
			}
			c.FilterOutUnusedPhases()
			assertion.Equal(test.wantPhases, c.Phases)
		})
	}
}
