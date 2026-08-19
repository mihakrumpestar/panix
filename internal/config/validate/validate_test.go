package validate

import (
	"fmt"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	installablepkg "github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/pkg/nixver"
	"github.com/mihakrumpestar/panix/pkg/xpath"
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

// buildDeclaredPresets builds a CustomOutputTypes ordered map with a single
// declared custom type, mirroring how the output_types section decodes.
func buildDeclaredPresets(typ string, preset installablepkg.Preset) installablepkg.CustomOutputTypes {
	declared := atomicorderedmap.New[string, installablepkg.Preset]()
	declared.Set(typ, preset)

	return declared
}

// TestValidateOutputTypes_AllKnownTypesPass verifies that all 7 known output
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
		"maidConfigurations",
	}

	// One flake with all 7 types.
	f := buildFleetWithTypes(t, knownTypes...)

	err := validateOutputTypes(f, nil)
	assert.NoError(t, err, "all 7 known output types should pass validation")
}

// TestValidateOutputTypes_UnknownTypeRejected verifies that an unknown output
// type is rejected with an error mentioning the type and listing known types.
func TestValidateOutputTypes_UnknownTypeRejected(t *testing.T) {
	t.Parallel()

	f := buildFleetWithTypes(t, "unknownConfigurations")

	err := validateOutputTypes(f, nil)
	require.Error(t, err, "unknown output type should be rejected")

	msg := err.Error()
	assert.Contains(t, msg, "unknown output type 'unknownConfigurations'",
		"error should name the unknown type")
	// The error should also list the known types so users can fix it.
	assert.Contains(t, msg, "nixosConfigurations",
		"error should list known types")
	// And point users at declaring custom types under output_types.
	assert.Contains(t, msg, "output_types",
		"error should mention custom types can be declared under output_types")
}

// TestValidateOutputTypes_MixedKnownAndUnknown verifies that when known and
// unknown types coexist, validation fails (one bad type fails the whole fleet).
func TestValidateOutputTypes_MixedKnownAndUnknown(t *testing.T) {
	t.Parallel()

	f := buildFleetWithTypes(t, "nixosConfigurations", "bogusType", "packages")

	err := validateOutputTypes(f, nil)
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

	err := validateOutputTypes(f, nil)
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
		_ = validateOutputTypes(f, nil)
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
		err := validateOutputTypes(f, nil)
		assert.NoError(t, err, "nil attr map should be skipped, not error")
	}, "nil attr map should be skipped, not panic")
}

// TestValidateOutputTypes_ErrorListsAllKnownTypes verifies that the error
// message for an unknown type includes every known type name, so the user
// knows what's valid.
func TestValidateOutputTypes_ErrorListsAllKnownTypes(t *testing.T) {
	t.Parallel()

	f := buildFleetWithTypes(t, "nope")

	err := validateOutputTypes(f, nil)
	require.Error(t, err)

	msg := err.Error()
	for _, known := range installablepkg.KnownOutputTypes() {
		assert.Contains(t, msg, known.String(),
			"error message should list known type %s, got: %s", known, msg)
	}
}

// TestValidateOutputTypes_DeclaredCustomTypePasses verifies that an output
// type declared under output_types passes validation even though it is not a
// built-in type.
func TestValidateOutputTypes_DeclaredCustomTypePasses(t *testing.T) {
	t.Parallel()

	declared := buildDeclaredPresets("colmenaConfigurations", installablepkg.Preset{
		IsSystemLevel: new(true),
	})

	f := buildFleetWithTypes(t, "colmenaConfigurations")

	err := validateOutputTypes(f, declared)
	assert.NoError(t, err, "declared custom output type should pass validation")
}

// TestValidateOutputTypes_CustomTypeMissingSystemLevel verifies that a custom
// output type declaration without system_level is rejected.
func TestValidateOutputTypes_CustomTypeMissingSystemLevel(t *testing.T) {
	t.Parallel()

	declared := buildDeclaredPresets("colmenaConfigurations", installablepkg.Preset{})

	f := buildFleetWithTypes(t, "colmenaConfigurations")

	err := validateOutputTypes(f, declared)
	require.Error(t, err, "custom type without system_level should be rejected")

	msg := err.Error()
	assert.Contains(t, msg, "colmenaConfigurations", "error should name the offending type")
	assert.Contains(t, msg, "system_level", "error should say system_level is required")
}

// TestValidateOutputTypes_CustomTypeCollidesWithBuiltin verifies that a custom
// type declaration whose name shadows a built-in type is rejected. Both the
// common case (nixosConfigurations) and the less obvious one (maidConfigurations,
// where a user might declare their own nix-maid type) are covered.
func TestValidateOutputTypes_CustomTypeCollidesWithBuiltin(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		typ  string
	}{
		{"nixosConfigurations collides with a built-in", "nixosConfigurations"},
		{"maidConfigurations collides with a built-in", "maidConfigurations"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			declared := buildDeclaredPresets(testCase.typ, installablepkg.Preset{
				IsSystemLevel: new(true),
			})

			f := buildFleetWithTypes(t, testCase.typ)

			err := validateOutputTypes(f, declared)
			require.Error(t, err, "custom type colliding with a built-in should be rejected")

			msg := err.Error()
			assert.Contains(t, msg, testCase.typ, "error should name the colliding type")
			assert.Contains(t, msg, "collides", "error should say the type collides with a built-in")
		})
	}
}

// TestValidateOutputTypes_DeclaredCustomTypeUnknownTypeError verifies that an
// undeclared type still errors when other custom types are declared (the
// declared set does not silently accept arbitrary type keys).
func TestValidateOutputTypes_DeclaredCustomTypeUnknownTypeError(t *testing.T) {
	t.Parallel()

	declared := buildDeclaredPresets("colmenaConfigurations", installablepkg.Preset{
		IsSystemLevel: new(true),
	})

	f := buildFleetWithTypes(t, "colmenaConfigurations", "notDeclaredType")

	err := validateOutputTypes(f, declared)
	require.Error(t, err, "undeclared unknown type should still be rejected")
	assert.Contains(t, err.Error(), "notDeclaredType")
}

// TestValidateOutputTypes_SupportedModesRequireDefaultMode verifies that a
// custom output type declaring activation_supported_modes must also declare
// activation_default_mode (the default mode drives rollback activation for the
// type), while declaring either field on its own is allowed.
func TestValidateOutputTypes_SupportedModesRequireDefaultMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		preset        installablepkg.Preset
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name: "supported modes without default mode rejected",
			preset: installablepkg.Preset{
				IsSystemLevel:   new(true),
				ActivationModes: []string{"switch", "boot"},
			},
			wantErr:       true,
			wantErrSubstr: "declares activation_supported_modes but not activation_default_mode",
		},
		{
			name: "supported modes with default mode accepted",
			preset: installablepkg.Preset{
				IsSystemLevel:         new(true),
				ActivationModes:       []string{"switch", "boot"},
				ActivationDefaultMode: "switch",
			},
			wantErr: false,
		},
		{
			name: "default mode without supported modes accepted",
			preset: installablepkg.Preset{
				IsSystemLevel:         new(true),
				ActivationDefaultMode: "switch",
			},
			wantErr: false,
		},
		{
			name: "neither field declared accepted",
			preset: installablepkg.Preset{
				IsSystemLevel: new(true),
			},
			wantErr: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			runSupportedModesRequireDefaultModeCase(t, testCase.preset, testCase.wantErr, testCase.wantErrSubstr)
		})
	}
}

// runSupportedModesRequireDefaultModeCase asserts the outcome of validating a
// declared "colmenaConfigurations" preset against the rule that supported
// activation modes require a declared default mode.
func runSupportedModesRequireDefaultModeCase(t *testing.T, preset installablepkg.Preset, wantErr bool, wantErrSubstr string) {
	t.Helper()

	declared := buildDeclaredPresets("colmenaConfigurations", preset)

	f := buildFleetWithTypes(t, "colmenaConfigurations")

	err := validateOutputTypes(f, declared)
	if wantErr {
		require.Error(t, err, "custom type with supported modes but no default mode should be rejected")

		msg := err.Error()
		assert.Contains(t, msg, "colmenaConfigurations", "error should name the offending type")
		assert.Contains(t, msg, wantErrSubstr, "error should explain the missing activation_default_mode")
		assert.Contains(t, msg, "set activation_default_mode to one of the supported modes",
			"error should tell the user how to fix it")
	} else {
		assert.NoError(t, err, "custom type declaration should pass validation")
	}
}

// TestValidateOutputTypes_DefaultModeMustBeSupported verifies that a custom
// output type whose activation_default_mode is not one of the declared
// activation_supported_modes is rejected, while a default that is a member of
// the supported modes passes.
func TestValidateOutputTypes_DefaultModeMustBeSupported(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		preset        installablepkg.Preset
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name: "default mode not in supported modes rejected",
			preset: installablepkg.Preset{
				IsSystemLevel:         new(true),
				ActivationModes:       []string{"switch", "boot"},
				ActivationDefaultMode: "test",
			},
			wantErr:       true,
			wantErrSubstr: "activation_default_mode 'test' is not in activation_supported_modes",
		},
		{
			name: "default mode in supported modes accepted",
			preset: installablepkg.Preset{
				IsSystemLevel:         new(true),
				ActivationModes:       []string{"switch", "boot"},
				ActivationDefaultMode: "boot",
			},
			wantErr: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			runDefaultModeMustBeSupportedCase(t, testCase.preset, testCase.wantErr, testCase.wantErrSubstr)
		})
	}
}

// runDefaultModeMustBeSupportedCase asserts the outcome of validating a
// declared "colmenaConfigurations" preset against the rule that the default
// activation mode must be one of the supported modes.
func runDefaultModeMustBeSupportedCase(t *testing.T, preset installablepkg.Preset, wantErr bool, wantErrSubstr string) {
	t.Helper()

	declared := buildDeclaredPresets("colmenaConfigurations", preset)

	f := buildFleetWithTypes(t, "colmenaConfigurations")

	err := validateOutputTypes(f, declared)
	if wantErr {
		require.Error(t, err, "default mode outside supported modes should be rejected")

		msg := err.Error()
		assert.Contains(t, msg, "colmenaConfigurations", "error should name the offending type")
		assert.Contains(t, msg, wantErrSubstr, "error should name the offending default mode")
	} else {
		assert.NoError(t, err, "custom type declaration should pass validation")
	}
}

// TestValidateOutputTypes_SetProfileRequiresProfilePath verifies that a custom
// output type declaring set_profile: true without a profile_path is rejected,
// while set_profile: true with a profile_path (and set_profile unset/absent)
// passes.
func TestValidateOutputTypes_SetProfileRequiresProfilePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		preset        installablepkg.Preset
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name: "set_profile true without profile_path rejected",
			preset: installablepkg.Preset{
				IsSystemLevel: new(true),
				SetProfile:    new(true),
			},
			wantErr:       true,
			wantErrSubstr: "declares set_profile: true but has no profile_path",
		},
		{
			name: "set_profile true with profile_path accepted",
			preset: installablepkg.Preset{
				IsSystemLevel: new(true),
				SetProfile:    new(true),
				ProfilePath:   "/nix/var/nix/profiles/system",
			},
			wantErr: false,
		},
		{
			name: "set_profile false without profile_path accepted",
			preset: installablepkg.Preset{
				IsSystemLevel: new(true),
				SetProfile:    new(false),
			},
			wantErr: false,
		},
		{
			name: "set_profile unset without profile_path accepted",
			preset: installablepkg.Preset{
				IsSystemLevel: new(true),
			},
			wantErr: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			runSetProfileRequiresProfilePathCase(t, testCase.preset, testCase.wantErr, testCase.wantErrSubstr)
		})
	}
}

// runSetProfileRequiresProfilePathCase asserts the outcome of validating a
// declared "colmenaConfigurations" preset against the rule that
// set_profile: true requires a profile_path.
func runSetProfileRequiresProfilePathCase(t *testing.T, preset installablepkg.Preset, wantErr bool, wantErrSubstr string) {
	t.Helper()

	declared := buildDeclaredPresets("colmenaConfigurations", preset)

	f := buildFleetWithTypes(t, "colmenaConfigurations")

	err := validateOutputTypes(f, declared)
	if wantErr {
		require.Error(t, err, "set_profile: true without profile_path should be rejected")

		msg := err.Error()
		assert.Contains(t, msg, "colmenaConfigurations", "error should name the offending type")
		assert.Contains(t, msg, wantErrSubstr, "error should explain the missing profile_path")
	} else {
		assert.NoError(t, err, "custom type declaration should pass validation")
	}
}

// newBuildModeMachine returns a machine whose SSH client is initialized with
// the given hostname. The machine is local iff hostname matches the local
// host "local-host". An explicit hostname avoids the SSH config alias lookup.
func newBuildModeMachine(t *testing.T, hostname string) *machine.Machine {
	t.Helper()

	machineI := &machine.Machine{}
	machineI.SSH.Hostname = hostname
	require.NoError(t, machineI.SSH.Init(hostname, "local-host", nixver.Info{}))

	return machineI
}

// newBuildModeInstallable returns an installable in the given build mode
// owning the machines in declaration order.
func newBuildModeInstallable(buildMode nix.BuildMode, machines ...*machine.Machine) *installablepkg.Installable {
	inst := &installablepkg.Installable{Nix: nix.NixConfig{BuildMode: buildMode}}
	inst.Machines = atomicorderedmap.New[string, *machine.Machine]()

	for i, m := range machines {
		inst.Machines.Set(fmt.Sprintf("machine-%d", i), m)
	}

	inst.Xpath = xpath.New("fleet", "test")

	return inst
}

// TestValidateBuildMode_LocalModeExempt verifies local mode never checks
// machines: it passes with none declared.
func TestValidateBuildMode_LocalModeExempt(t *testing.T) {
	t.Parallel()

	inst := newBuildModeInstallable(nix.BuildModeLocal)

	assert.Empty(t, validateBuildMode(inst, "test.xpath", nil))
}

// TestValidateBuildMode_RemoteNoMachinesRejected verifies remote mode
// requires at least 1 machine.
func TestValidateBuildMode_RemoteNoMachinesRejected(t *testing.T) {
	t.Parallel()

	inst := newBuildModeInstallable(nix.BuildModeRemote)

	assert.Equal(t,
		[]string{"test.xpath: remote mode requires at least 1 machine"},
		validateBuildMode(inst, "test.xpath", nil))
}

// TestValidateBuildMode_RemoteFirstMachineRemote verifies remote mode passes
// when the pinned builder (first declared machine) is remote.
func TestValidateBuildMode_RemoteFirstMachineRemote(t *testing.T) {
	t.Parallel()

	inst := newBuildModeInstallable(nix.BuildModeRemote,
		newBuildModeMachine(t, "10.0.0.1"),
		newBuildModeMachine(t, "10.0.0.2"))

	assert.Empty(t, validateBuildMode(inst, "test.xpath", nil))
}

// TestValidateBuildMode_RemoteFirstMachineLocalRejected verifies the pinned
// builder must not be the local machine.
func TestValidateBuildMode_RemoteFirstMachineLocalRejected(t *testing.T) {
	t.Parallel()

	inst := newBuildModeInstallable(nix.BuildModeRemote,
		newBuildModeMachine(t, "local-host"),
		newBuildModeMachine(t, "10.0.0.2"))

	assert.Equal(t,
		[]string{"test.xpath: remote mode requires the first machine to be remote (not local)"},
		validateBuildMode(inst, "test.xpath", nil))
}
