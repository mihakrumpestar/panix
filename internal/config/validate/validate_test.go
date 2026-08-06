package validate

import (
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	installablepkg "github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildFleetWithTypes builds a minimal Fleet containing one flake with one
// installable per given output type. Each installable is Init'd so its Xpath
// is populated (validateOutputTypes reads installable.Xpath for error
// messages). This avoids touching the network — validateOutputTypes is a pure
// function that only checks IsKnown() on the type key.
func buildFleetWithTypes(t *testing.T, types ...string) *fleet.Fleet {
	t.Helper()

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()

	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Installables = atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]]()

	for _, typ := range types {
		inst := &installablepkg.Installable{}
		inst.Logs = logs.New()
		// Machines is required by struct tags but validateOutputTypes doesn't
		// access it; Init doesn't either. Leave it nil for this minimal build.

		attrMap := atomicorderedmap.New[string, *installablepkg.Installable]()
		attrMap.Set("cfg0", inst)
		flakeObj.Installables.Set(typ, attrMap)
	}

	flakesMap.Set("flake0", flakeObj)

	return &fleet.Fleet{Flakes: flakesMap}
}

// TestValidateOutputTypes_AllKnownTypesPass verifies that all 6 known output
// types pass validation without error. This is a pure-function test — no nix
// invocations.
func TestValidateOutputTypes_AllKnownTypesPass(t *testing.T) {
	t.Parallel()

	knownTypes := []string{
		"nixosConfigurations",
		"darwinConfigurations",
		"systemConfigs",
		"homeConfigurations",
		"nixOnDroidConfigurations",
		"packages",
	}

	// One flake with all 6 types.
	f := buildFleetWithTypes(t, knownTypes...)

	err := validateOutputTypes(f)
	assert.NoError(t, err, "all 6 known output types should pass validation")
}

// TestValidateOutputTypes_UnknownTypeRejected verifies that an unknown output
// type is rejected with an error mentioning the type and listing known types.
func TestValidateOutputTypes_UnknownTypeRejected(t *testing.T) {
	t.Parallel()

	f := buildFleetWithTypes(t, "unknownConfigurations")

	err := validateOutputTypes(f)
	require.Error(t, err, "unknown output type should be rejected")

	msg := err.Error()
	assert.Contains(t, msg, "unknown output type 'unknownConfigurations'",
		"error should name the unknown type")
	// The error should also list the known types so users can fix it.
	assert.Contains(t, msg, "nixosConfigurations",
		"error should list known types")
}

// TestValidateOutputTypes_MixedKnownAndUnknown verifies that when known and
// unknown types coexist, validation fails (one bad type fails the whole fleet).
func TestValidateOutputTypes_MixedKnownAndUnknown(t *testing.T) {
	t.Parallel()

	f := buildFleetWithTypes(t, "nixosConfigurations", "bogusType", "packages")

	err := validateOutputTypes(f)
	require.Error(t, err, "presence of an unknown type should fail validation")
	assert.Contains(t, err.Error(), "bogusType")
}

// TestValidateOutputTypes_EmptyFleet verifies that a fleet with no flakes
// passes validation (vacuously true — nothing to check).
func TestValidateOutputTypes_EmptyFleet(t *testing.T) {
	t.Parallel()

	f := &fleet.Fleet{
		Flakes: atomicorderedmap.New[string, *flake.Flake](),
	}

	err := validateOutputTypes(f)
	assert.NoError(t, err, "empty fleet should pass validation")
}

// TestValidateOutputTypes_NilInstallableSkipped verifies that a nil installable
// pointer is skipped gracefully rather than panicking. The validation loop
// guards against nil installables (see flake.go:191-194).
func TestValidateOutputTypes_NilInstallableSkipped(t *testing.T) {
	t.Parallel()

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()
	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Installables = atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]]()

	// A known type with a nil installable pointer — must not panic.
	attrMap := atomicorderedmap.New[string, *installablepkg.Installable]()
	attrMap.Set("cfg0", nil)
	flakeObj.Installables.Set("nixosConfigurations", attrMap)
	flakesMap.Set("flake0", flakeObj)

	f := &fleet.Fleet{Flakes: flakesMap}

	assert.NotPanics(t, func() {
		_ = validateOutputTypes(f)
	}, "nil installable should be skipped, not panic")
}

// TestValidateOutputTypes_NilAttrMapSkipped verifies that a nil attribute map
// (the second-level map) is skipped gracefully.
func TestValidateOutputTypes_NilAttrMapSkipped(t *testing.T) {
	t.Parallel()

	flakesMap := atomicorderedmap.New[string, *flake.Flake]()
	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Installables = atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]]()

	// Set a type key with a nil attr map.
	flakeObj.Installables.Set("nixosConfigurations", nil)
	flakesMap.Set("flake0", flakeObj)

	f := &fleet.Fleet{Flakes: flakesMap}

	assert.NotPanics(t, func() {
		err := validateOutputTypes(f)
		assert.NoError(t, err, "nil attr map should be skipped, not error")
	}, "nil attr map should be skipped, not panic")
}

// TestValidateOutputTypes_ErrorListsAllKnownTypes verifies that the error
// message for an unknown type includes every known type name, so the user
// knows what's valid.
func TestValidateOutputTypes_ErrorListsAllKnownTypes(t *testing.T) {
	t.Parallel()

	f := buildFleetWithTypes(t, "nope")

	err := validateOutputTypes(f)
	require.Error(t, err)

	msg := err.Error()
	for _, known := range installablepkg.KnownOutputTypes() {
		assert.Contains(t, msg, known.String(),
			"error message should list known type %s, got: %s", known, msg)
	}
}
