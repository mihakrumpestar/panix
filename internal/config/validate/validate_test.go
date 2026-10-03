package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
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

// transferSourceValidator mirrors the validator the loader builds: registered
// path validators plus required-struct semantics.
func transferSourceValidator() *validator.Validate {
	validate := validator.New(validator.WithRequiredStructEnabled())
	registerPathValidators(validate)

	return validate
}

// transferSourceRoot wraps a TransferSource so ValidateStructTags, the entry
// point LoadConfig uses, can validate it exactly like the loader does.
type transferSourceRoot struct {
	Source attributes.TransferSource
}

// TestTransferSourceLocalPathDirectoryRegression guards that an existing
// directory local_path passes ValidateStructTags, the entry point the loader
// uses: rsync and the docs support directory sources, so no path validator may
// reject directories here again.
func TestTransferSourceLocalPathDirectoryRegression(t *testing.T) {
	t.Parallel()

	secretDir := t.TempDir()

	root := transferSourceRoot{
		Source: attributes.TransferSource{LocalPath: secretDir, RemotePath: "/etc/secrets"},
	}

	err := ValidateStructTags(&root, &fleet.Fleet{}, nil, flags.ValidateFlags{}, 0)
	require.NoError(t, err, "an existing directory local_path must pass validation")
}

// TestTransferSourceRequiredWithout covers the local_path/command pair: either
// source alone passes, both together pass, and neither produces a readable
// "one of" error instead of the raw validator tag.
func TestTransferSourceRequiredWithout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "secret.key")
	require.NoError(t, os.WriteFile(existingFile, []byte("secret"), 0600))

	validate := transferSourceValidator()

	tests := []struct {
		name    string
		source  attributes.TransferSource
		wantErr bool
	}{
		{
			name:   "command only passes",
			source: attributes.TransferSource{Command: "echo secret", RemotePath: "/etc/secret"},
		},
		{
			name:   "local path only passes",
			source: attributes.TransferSource{LocalPath: existingFile, RemotePath: "/etc/secret"},
		},
		{
			name: "both set passes",
			source: attributes.TransferSource{
				LocalPath:  existingFile,
				Command:    "cat $PANIX_SECRET_LOCAL_PATH",
				RemotePath: "/etc/secret",
			},
		},
		{
			name:    "neither set fails",
			source:  attributes.TransferSource{RemotePath: "/etc/secret"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate.Struct(tt.source)
			if !tt.wantErr {
				assert.NoError(t, err)

				return
			}

			require.Error(t, err)

			msg := humanizeValidationErrors(err)
			assert.Contains(t, msg, "one of command or local_path is required")
			assert.NotContains(t, msg, "failed validation")
		})
	}
}

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

// rollbackRoot wraps Attributes so ValidateStructTags validates the rollback
// enum exactly like the loader does over the full configuration.
type rollbackRoot struct {
	attributes.Attributes
}

// buildRollbackFleet builds a one-flake one-installable one-machine fleet with
// the given attributes at each level, with xpaths matching a real Init'd tree
// so rule violations report meaningful paths.
func buildRollbackFleet(fleetAttrs, flakeAttrs, instAttrs, machAttrs attributes.Attributes) *fleet.Fleet {
	flakesMap := atomicorderedmap.New[string, *flake.Flake]()

	flakeObj := &flake.Flake{URL: "github:test/test"}
	flakeObj.Logs = logs.New()
	flakeObj.Attributes = flakeAttrs
	flakeObj.Xpath = xpath.New("my-flake")
	flakeObj.Installables = atomicorderedmap.New[string, *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]]()

	inst := &installablepkg.Installable{}
	inst.Logs = logs.New()
	inst.Attributes = instAttrs
	inst.Xpath = xpath.New("my-flake").NewXpathWithAppend("nixosConfigurations/my-config")
	inst.Machines = atomicorderedmap.New[string, *machine.Machine]()

	mach := &machine.Machine{}
	mach.Attributes = machAttrs
	mach.Xpath = xpath.New("my-flake").NewXpathWithAppend("nixosConfigurations/my-config").NewXpathWithAppend("m0")
	inst.Machines.Set("m0", mach)

	attrMap := atomicorderedmap.New[string, *installablepkg.Installable]()
	attrMap.Set("cfg0", inst)
	flakeObj.Installables.Set("nixosConfigurations", attrMap)

	flakesMap.Set("flake0", flakeObj)

	return &fleet.Fleet{Flakes: flakesMap, Attributes: fleetAttrs}
}

// TestValidateRollbackAttributes covers the Activation Guard tier rules from
// docs/design/activation-guard.md section 2 at the machine level (the level
// deployment runs at), plus fleet- and flake-level enforcement. The check
// budget rule is intentionally absent: per-check timeouts are guardian
// internal constants, not configuration.
func TestValidateRollbackAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fleetAttrs attributes.Attributes
		flakeAttrs attributes.Attributes
		instAttrs  attributes.Attributes
		machAttrs  attributes.Attributes
		wantErr    bool
		wantMsg    string
	}{
		{
			name:    "clean config passes",
			wantErr: false,
		},
		{
			name:      "health_checks with magic passes",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackMagic, HealthChecks: []string{"curl localhost"}},
			wantErr:   false,
		},
		{
			name:      "health_checks with off fails",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackOff, HealthChecks: []string{"curl localhost"}},
			wantErr:   true,
			wantMsg:   "health_checks requires rollback: magic",
		},
		{
			name:      "health_checks with unset tier fails",
			machAttrs: attributes.Attributes{HealthChecks: []string{"curl localhost"}},
			wantErr:   true,
			wantMsg:   "health_checks requires rollback: magic",
		},
		{
			name:      "health_checks_local with auto passes",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackAuto, HealthChecksLocal: []string{"systemctl is-active app"}},
			wantErr:   false,
		},
		{
			name:      "health_checks_local with magic passes",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackMagic, HealthChecksLocal: []string{"systemctl is-active app"}},
			wantErr:   false,
		},
		{
			name:      "health_checks_local with off fails",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackOff, HealthChecksLocal: []string{"systemctl is-active app"}},
			wantErr:   true,
			wantMsg:   "health_checks_local requires rollback: auto or magic",
		},
		{
			name:      "health_checks_local with unset tier fails",
			machAttrs: attributes.Attributes{HealthChecksLocal: []string{"systemctl is-active app"}},
			wantErr:   true,
			wantMsg:   "health_checks_local requires rollback: auto or magic",
		},
		{
			name:      "rollback_confirm_timeout with magic passes",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackMagic, RollbackConfirmTimeout: 90 * time.Second},
			wantErr:   false,
		},
		{
			name:      "rollback_confirm_timeout with auto fails",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackAuto, RollbackConfirmTimeout: 90 * time.Second},
			wantErr:   true,
			wantMsg:   "rollback_confirm_timeout requires rollback: magic",
		},
		{
			name:      "rollback_confirm_timeout with off fails",
			machAttrs: attributes.Attributes{Rollback: attributes.RollbackOff, RollbackConfirmTimeout: 90 * time.Second},
			wantErr:   true,
			wantMsg:   "rollback_confirm_timeout requires rollback: magic",
		},
		{
			name:      "rollback_confirm_timeout with unset tier fails",
			machAttrs: attributes.Attributes{RollbackConfirmTimeout: 90 * time.Second},
			wantErr:   true,
			wantMsg:   "rollback_confirm_timeout requires rollback: magic",
		},
		{
			name:       "fleet level rule fires with fleet path",
			fleetAttrs: attributes.Attributes{HealthChecks: []string{"curl localhost"}},
			wantErr:    true,
			wantMsg:    "fleet: health_checks requires rollback: magic",
		},
		{
			name:       "flake level rule fires with flake path",
			flakeAttrs: attributes.Attributes{RollbackConfirmTimeout: 30 * time.Second},
			wantErr:    true,
			wantMsg:    "my-flake: rollback_confirm_timeout requires rollback: magic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := buildRollbackFleet(tt.fleetAttrs, tt.flakeAttrs, tt.instAttrs, tt.machAttrs)

			err := validateRollbackAttributes(f)
			if !tt.wantErr {
				assert.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

// TestValidateRollbackEnumRejected verifies the rollback tier enum through the
// full ValidateStructTags entry point: an unknown tier value fails with the
// humanized oneof message naming the valid tiers.
func TestValidateRollbackEnumRejected(t *testing.T) {
	t.Parallel()

	root := &rollbackRoot{Attributes: attributes.Attributes{Rollback: "banana"}}

	err := ValidateStructTags(root, &fleet.Fleet{}, nil, flags.ValidateFlags{}, 0)
	require.Error(t, err, "an unknown rollback tier must fail validation")

	assert.Contains(t, err.Error(), "must be one of [off auto magic]")
}

// TestValidateHealthChecksNeverRunInBootMode covers the inert-checks rule
// (validateInertHealthChecks): remote health_checks are a config error on a
// boot-mode installable whose transaction is gate-ineligible (the full tier's
// boot-mode commit is empty by design, so magic degrades to auto and the
// checks would never run), while non-boot modes, empty checks, and the
// guardian-side health_checks_local stay valid.
func TestValidateHealthChecksNeverRunInBootMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		activationMode    string // installable-level override; empty uses the preset default
		presetType        installablepkg.FlakeOutputType
		healthChecks      []string
		healthChecksLocal []string
		wantErr           bool
		wantMsg           string
	}{
		{
			name:           "boot mode with magic and health_checks is an inert config error",
			activationMode: "boot",
			presetType:     installablepkg.FlakeOutputType("nixosConfigurations"),
			healthChecks:   []string{"curl localhost"},
			wantErr:        true,
			wantMsg:        "health_checks never run in boot mode",
		},
		{
			name:           "boot mode with preset-default mode override and checks fails",
			activationMode: "boot",
			presetType:     installablepkg.FlakeOutputType("nixosConfigurations"),
			healthChecks:   []string{"curl localhost"},
			wantErr:        true,
			wantMsg:        "use rollback: auto without checks, or a non-boot mode for checks",
		},
		{
			name:           "switch mode under magic passes (inert rule is boot-only)",
			activationMode: "switch",
			presetType:     installablepkg.FlakeOutputType("nixosConfigurations"),
			healthChecks:   []string{"curl localhost"},
			wantErr:        false,
		},
		{
			name:           "boot mode without health_checks passes",
			activationMode: "boot",
			presetType:     installablepkg.FlakeOutputType("nixosConfigurations"),
			wantErr:        false,
		},
		{
			name:              "boot mode health_checks_local stay valid (not gate-gated)",
			activationMode:    "boot",
			presetType:        installablepkg.FlakeOutputType("nixosConfigurations"),
			healthChecksLocal: []string{"systemctl is-active app"},
			wantErr:           false,
		},
		{
			name:           "non-full tier boot mode (home-manager) is not gate-ineligible",
			activationMode: "boot",
			presetType:     installablepkg.FlakeOutputType("homeConfigurations"),
			healthChecks:   []string{"curl localhost"},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Magic everywhere above installable so the fleet/flake/machine
			// tier rules never fire: this table isolates the boot-mode rule.
			magic := attributes.Attributes{Rollback: attributes.RollbackMagic}
			f := buildRollbackFleet(magic, magic, magic, magic)
			inst := onlyInstallable(f)
			require.NotNil(t, inst)

			inst.Preset = buildPresetForType(t, tt.presetType)
			inst.ActivationMode = tt.activationMode
			inst.HealthChecks = tt.healthChecks
			inst.HealthChecksLocal = tt.healthChecksLocal

			err := validateRollbackAttributes(f)
			if !tt.wantErr {
				assert.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

// onlyInstallable returns the single fixture installable of a
// buildRollbackFleet fleet.
func onlyInstallable(f *fleet.Fleet) *installablepkg.Installable {
	var inst *installablepkg.Installable

	f.Flakes.ForEach(func(_ string, flakeV *flake.Flake) bool {
		return flakeV.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]) bool {
			if attrMap == nil {
				return true
			}

			attrMap.ForEach(func(_ string, i *installablepkg.Installable) bool {
				inst = i

				return true
			})

			return true
		})
	})

	return inst
}

// buildPresetForType returns the preset row for a built-in output type.
func buildPresetForType(t *testing.T, typ installablepkg.FlakeOutputType) installablepkg.Preset {
	t.Helper()

	preset, ok := installablepkg.PresetForType(typ)
	require.True(t, ok, "unknown preset type %q in fixture", typ)

	return preset
}
