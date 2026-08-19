package transfer

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunPhase_NilMetaBuildErrorsCleanly verifies the guard for transfer
// without build (e.g. --skip-phases=build): MetaBuild is produced by the
// Build phase, so a nil MetaBuild must produce an actionable error instead
// of the nil-pointer panic it caused before the guard.
func TestRunPhase_NilMetaBuildErrorsCleanly(t *testing.T) {
	t.Parallel()

	handler := Handler{}

	leaf := &fleet.FleetLeaf{
		Flake:       &flake.Flake{},
		Installable: &installable.Installable{}, // MetaBuild intentionally nil
		Machine:     &machine.Machine{},
	}

	// The guard fires before the executioner is used, so nil exc is safe.
	err := handler.RunPhase(nil, leaf)

	require.Error(t, err)
	assert.EqualError(t, err, "transfer requires the build phase to have produced a closure")
}
