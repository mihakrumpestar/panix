package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/testutil"
	"github.com/mihakrumpestar/panix/internal/workflow/phase"
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

				return fk.Fleet(fk.Flake(fk.Configuration(fk.Machine(), fk.Machine())))
			},
			false, false,
		},
		{
			"machine with secrets",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.Machine(), fk.MachineWithSecrets(2))))
			},
			true, false,
		},
		{
			"machine with bootstrap ssh",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.Machine(), fk.MachineWithBootstrapSSH())))
			},
			false, true,
		},
		{
			"machine with force bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.MachineWithForceBootstrap())))
			},
			false, true,
		},
		{
			"both secrets and bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.MachineWithSecrets(1), fk.MachineWithBootstrapSSH())))
			},
			true, true,
		},
		{
			"secrets in different flake",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(
					fk.Flake(fk.Configuration(fk.MachineWithSecrets(1))),
					fk.Flake(fk.Configuration(fk.MachineWithBootstrapSSH())),
				)
			},
			true, true,
		},
		{
			"empty fleet",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration()))
			},
			false, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			result := hasRequiredPhases(tt.buildFleet())
			assertion.Equal(tt.wantSecrets, result.Secrets)
			assertion.Equal(tt.wantBootstrap, result.Bootstrap)
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

				return fk.Fleet(fk.Flake(fk.Configuration(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
		{
			"removes bootstrap when no bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
		{
			"removes both unused phases",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
		{
			"keeps secrets when machine has secrets",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.MachineWithSecrets(1))))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Secrets, phase.Activate},
		},
		{
			"keeps bootstrap when machine has bootstrap",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.MachineWithBootstrapSSH())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Activate},
		},
		{
			"keeps all phases when both needed",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.MachineWithSecrets(1), fk.MachineWithBootstrapSSH())))
			},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Bootstrap, phase.Build, phase.Secrets, phase.Activate},
		},
		{
			"does not remove phases not in list",
			func() *fleet.Fleet {
				fk := testutil.NewFaker()

				return fk.Fleet(fk.Flake(fk.Configuration(fk.Machine())))
			},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
			[]phase.Phase{phase.Inspect, phase.Build, phase.Activate},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			c := &Config{
				Fleet:  tt.buildFleet(),
				Phases: tt.inputPhases,
			}
			c.FilterOutUnusedPhases()
			assertion.Equal(tt.wantPhases, c.Phases)
		})
	}
}
