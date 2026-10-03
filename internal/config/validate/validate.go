package validate

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	installablepkg "github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops/guard"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicorderedmap"
	"github.com/pkg/errors"
	"github.com/stoewer/go-strcase"
)

// ValidateStructTags runs struct-tag validation over the whole configuration
// (via reflection on conf, which must be the *config.Config value), followed
// by the path, flake, build-mode, and output-type checks. The fleet and its
// related settings are passed as explicit leaf-typed parameters rather than
// derived from conf, because this package cannot import the config package
// (the import direction is config -> validate).
func ValidateStructTags(
	conf any,
	fl *fleet.Fleet, //nolint:varnamelen
	outputTypes installablepkg.CustomOutputTypes,
	vFlags flags.ValidateFlags,
	flakeValidationTimeout time.Duration,
) error {
	validate := validator.New(validator.WithRequiredStructEnabled(), validator.WithPrivateFieldValidation(), validator.WithRequiredStructEnabled())

	registerPathValidators(validate)

	err := validate.Struct(conf)
	if err != nil {
		return errors.New(humanizeValidationErrors(err))
	}

	err = validatePaths(fl, vFlags)
	if err != nil {
		return err
	}

	if vFlags.Validate.Flakes {
		err = validateFlakes(fl, flakeValidationTimeout)
		if err != nil {
			return errors.Wrap(err, "invalid flakes configuration")
		}
	}

	err = validateBuildModes(fl)
	if err != nil {
		return errors.Wrap(err, "invalid build mode configuration")
	}

	err = validateRollbackAttributes(fl)
	if err != nil {
		return errors.Wrap(err, "invalid rollback configuration")
	}

	err = validateOutputTypes(fl, outputTypes)
	if err != nil {
		return errors.Wrap(err, "invalid output type configuration")
	}

	return nil
}

func humanizeValidationErrors(err error) string {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return err.Error()
	}

	seen := make(map[string]bool, len(validationErrors))

	var builder strings.Builder
	builder.WriteString("configuration validation errors:\n")

	for _, fe := range validationErrors {
		path := humanizePath(fe.Namespace())
		msg := humanizeTagMessage(fe)
		key := fmt.Sprintf("%s: %s", path, msg)

		if !seen[key] {
			seen[key] = true
			fmt.Fprintf(&builder, "  - %s\n", key)
		}
	}

	return builder.String()
}

// skipParts are Go type/field names that appear in validator namespaces but aren't meaningful in user-facing paths.
var skipParts = map[string]bool{
	"Config": true, "Flake": true, "Installable": true, "Machine": true, "Attributes": true, "Values": true,
}

func humanizePath(namespace string) string {
	parts := strings.Split(namespace, ".")

	var result []string

	for _, part := range parts {
		if part == "" || skipParts[part] {
			continue
		}

		result = append(result, strcase.SnakeCase(part))
	}

	return strings.Join(result, ".")
}

//nolint:cyclop // one branch per validator tag, tags are independent
func humanizeTagMessage(fieldError validator.FieldError) string {
	switch fieldError.Tag() {
	case "required":
		return "is required"
	case "filepath":
		return fmt.Sprintf("invalid file path: %v", fieldError.Value())
	case "abspath":
		return fmt.Sprintf("must be an absolute path, got: %v", fieldError.Value())
	case "dir_exists":
		return fmt.Sprintf("directory does not exist: %v", fieldError.Value())
	case "url":
		return fmt.Sprintf("must be a valid URL, got: %v", fieldError.Value())
	case "uri":
		return fmt.Sprintf("must be a valid URI, got: %v", fieldError.Value())
	case "required_without":
		return humanizeRequiredWithout(fieldError)
	case "oneof":
		return fmt.Sprintf("must be one of [%s], got: %v", fieldError.Param(), fieldError.Value())
	case "dive":
		return "contains invalid elements"
	default:
		return fmt.Sprintf("failed validation '%s' (value: %v)", fieldError.Tag(), fieldError.Value())
	}
}

// humanizeRequiredWithout renders a required_without error as a readable
// choice: the field and the field(s) it depends on are sorted so symmetric
// declarations (a required_without b, b required_without a) produce the same
// message.
func humanizeRequiredWithout(fieldError validator.FieldError) string {
	names := []string{strcase.SnakeCase(fieldError.Field())}
	for name := range strings.FieldsSeq(fieldError.Param()) {
		names = append(names, strcase.SnakeCase(name))
	}

	slices.Sort(names)

	return fmt.Sprintf("one of %s is required", strings.Join(names, " or "))
}

func validateBuildModes(f *fleet.Fleet) error {
	var errs []string

	for _, flakePair := range f.Flakes.Pairs() {
		flakePair.Value.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]) bool {
			if attrMap == nil {
				return true
			}

			attrMap.ForEach(func(_ string, installable *installablepkg.Installable) bool {
				errs = validateBuildMode(installable, installable.Xpath.String(), errs)

				return true
			})

			return true
		})
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func validateBuildMode(out *installablepkg.Installable, outPath string, errs []string) []string {
	if out.Nix.BuildMode != nix.BuildModeRemote {
		return errs
	}

	// The builder is the first declared machine (Installable.RemoteBuilder);
	// build (--store) and transfer (--from) both pin to it.
	builder := out.RemoteBuilder()

	if builder == nil {
		errs = append(errs, outPath+": remote mode requires at least 1 machine")
	}

	if builder != nil && builder.SSH.IsLocal() {
		errs = append(errs, outPath+": remote mode requires the first machine to be remote (not local)")
	}

	return errs
}

// validateOutputTypes checks that every installable output type is either a
// built-in type (IsKnown) or declared under output_types, and that the
// declared custom types are well-formed: they must not collide with a built-in
// name, must declare whether they are system-level, and must declare an
// activation default mode when they declare supported activation modes. When
// both modes and a default mode are declared, the default must be one of the
// supported modes, and set_profile: true requires a profile_path.
func validateOutputTypes(fleetConfig *fleet.Fleet, declaredPresets installablepkg.CustomOutputTypes) error {
	var errs []string

	knownTypes := installablepkg.KnownOutputTypes()

	knownTypeStrs := make([]string, len(knownTypes))
	for i, t := range knownTypes {
		knownTypeStrs[i] = t.String()
	}

	errs = append(errs, validateDeclaredPresets(declaredPresets)...)

	for _, flakePair := range fleetConfig.Flakes.Pairs() {
		flakePair.Value.Installables.ForEach(func(typeKey string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]) bool {
			if attrMap == nil {
				return true
			}

			attrMap.ForEach(func(nameKey string, installable *installablepkg.Installable) bool {
				if installable == nil {
					return true
				}

				typ := installablepkg.FlakeOutputType(typeKey)
				if typ.IsKnown() {
					return true
				}

				if declaredPresets != nil {
					_, declared := declaredPresets.Get(typeKey)
					if declared {
						return true
					}
				}

				errs = append(errs, fmt.Sprintf(
					"%s: unknown output type '%s', known types: %s. "+
						"Custom output types can be declared under 'output_types'",
					installable.Xpath.String(), typeKey, strings.Join(knownTypeStrs, ", ")))

				return true
			})

			return true
		})
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

// validateDeclaredPresets checks that the custom output types declared under
// output_types are well-formed, returning the accumulated error messages: a
// declared type must not collide with a built-in name, must set 'system_level',
// and must declare 'activation_default_mode' when it declares supported
// activation modes (the default must be one of the supported modes).
// set_profile: true requires a profile_path. Entries in
// activation_non_mutating_modes and activation_profile_skip_modes must be
// declared in activation_supported_modes.
func validateDeclaredPresets(declaredPresets installablepkg.CustomOutputTypes) []string {
	var errs []string

	declaredPresets.ForEach(func(typeKey string, preset installablepkg.Preset) bool {
		typ := installablepkg.FlakeOutputType(typeKey)
		if typ.IsKnown() {
			errs = append(errs, fmt.Sprintf("output_types: '%s' collides with a built-in output type", typ))
		}

		if preset.IsSystemLevel == nil {
			errs = append(errs, fmt.Sprintf("output_types: '%s' must set 'system_level'", typ))
		}

		// ActivationDefaultMode drives the rollback activation mode for the
		// type, so declaring supported modes without a default would leave
		// rollback unable to pick one. When both are declared, the default
		// must be one of the supported modes, otherwise activation would
		// request a mode the type does not support.
		if len(preset.ActivationModes) > 0 {
			if preset.ActivationDefaultMode == "" {
				errs = append(errs, fmt.Sprintf(
					"output_types: '%s' declares activation_supported_modes but not activation_default_mode, "+
						"set activation_default_mode to one of the supported modes",
					typ,
				))
			} else if !slices.Contains(preset.ActivationModes, preset.ActivationDefaultMode) {
				errs = append(errs, fmt.Sprintf(
					"output_types: '%s' activation_default_mode '%s' is not in activation_supported_modes",
					typ, preset.ActivationDefaultMode,
				))
			}
		}

		// set_profile: true only makes sense with a profile path to set; a
		// type that sets a profile must know where to set it.
		if preset.SetProfile != nil && *preset.SetProfile && preset.ProfilePath == "" {
			errs = append(errs, fmt.Sprintf("output_types: '%s' declares set_profile: true but has no profile_path", typ))
		}

		// The mode semantic lists reference modes the activation script
		// understands; entries outside activation_supported_modes would
		// silently never match at runtime.
		errs = append(errs, validateModeSubset(typ, preset.NonMutatingModes, preset.ActivationModes, "activation_non_mutating_modes")...)
		errs = append(errs, validateModeSubset(typ, preset.ProfileSkipModes, preset.ActivationModes, "activation_profile_skip_modes")...)

		return true
	})

	return errs
}

// validateModeSubset checks that every entry of a mode semantic list
// (e.g. activation_non_mutating_modes) is declared in the type's supported
// activation modes, returning an error message per offending entry.
func validateModeSubset(typ installablepkg.FlakeOutputType, modes, supportedModes []string, listName string) []string {
	var errs []string

	for _, mode := range modes {
		if !slices.Contains(supportedModes, mode) {
			errs = append(errs, fmt.Sprintf(
				"output_types: '%s' %s entry '%s' is not in activation_supported_modes",
				typ, listName, mode,
			))
		}
	}

	return errs
}

// validateRollbackAttributes enforces the Activation Guard tier rules at every
// attribute level (fleet, flake, installable, machine), per
// docs/design/activation-guard.md section 2:
//
//   - health_checks non-empty requires rollback: magic, and a mode where the
//     transaction is gate-eligible (boot mode degrades magic to auto, so the
//     tier-level rule pairs with the per-installable boot-mode rule below)
//   - health_checks_local non-empty requires rollback: auto or magic
//   - rollback_confirm_timeout set requires rollback: magic
//
// Values are post-merge (inheritance already applied), so an empty rollback
// means no level set it and behaves like off. The rollback enum itself is
// covered by the oneof struct tag validated over the whole configuration.
//
// The tier-attribute checks walk the attribute levels; the remote checks' one
// mode-dependent inertness rule (health_checks on a boot-mode transaction,
// spec 5) needs the installable, whose activation mode never rides
// Attributes, so it is enforced separately per installable below.
func validateRollbackAttributes(f *fleet.Fleet) error {
	var errs []string

	errs = validateRollbackTier("fleet", f.Attributes, errs)

	f.Flakes.ForEach(func(_ string, flakeV *flake.Flake) bool {
		if flakeV == nil {
			return true
		}

		errs = validateRollbackTier(flakeV.Xpath.String(), flakeV.Attributes, errs)

		flakeV.Installables.ForEach(func(_ string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]) bool {
			if attrMap == nil {
				return true
			}

			attrMap.ForEach(func(_ string, installable *installablepkg.Installable) bool {
				if installable == nil {
					return true
				}

				errs = validateRollbackTier(installable.Xpath.String(), installable.Attributes, errs)

				errs = validateInertHealthChecks(installable, errs)

				installable.Machines.ForEach(func(_ string, mach *machine.Machine) bool {
					if mach == nil {
						return true
					}

					errs = validateRollbackTier(mach.Xpath.String(), mach.Attributes, errs)

					return true
				})

				return true
			})

			return true
		})

		return true
	})

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

// validateInertHealthChecks rejects health_checks where the effective gate for
// the installable's activation mode is auto by gate ineligibility rather than
// by tier choice (docs/design/activation-guard.md section 2, spec 2): boot
// mode's full-tier commit is empty by design (the profile set moves to
// pre-start and the bootloader install belongs to the activation), so the
// magic tier degrades to auto at composition, the confirm loop never arms,
// and the panix-side remote checks would never run. There is no live new
// system to confirm anyway: the running system stays the old generation until
// reboot. Only the remote checks are gate-gated; health_checks_local run in
// the guardian's CHECKING phase regardless of the gate and stay valid in boot
// mode. The mode comes from the same precedence the activate handler applies
// at workflow time (installable activation_mode override, then the preset
// default); the CLI override is workflow runtime state and stays out of
// config-time validation.
func validateInertHealthChecks(installable *installablepkg.Installable, errs []string) []string {
	mode := installable.Preset.ActivationDefaultMode
	if installable.ActivationMode != "" {
		mode = installable.ActivationMode
	}

	if !guard.BootModeGateIneligible(installable.Preset, mode) {
		return errs
	}

	if len(installable.HealthChecks) == 0 {
		return errs
	}

	return append(errs, fmt.Sprintf(
		"%s: health_checks never run in boot mode: the activation guard degrades to the auto tier because "+ //nolint:lll
			"boot mode has nothing to confirm (the new system is not live until reboot); "+ //nolint:lll
			"use rollback: auto without checks, or a non-boot mode for checks", //nolint:lll
		installable.Xpath.String(),
	))
}

// validateRollbackTier appends the tier-rule violations of a single attribute
// level to errs, prefixed with the level's xpath (or "fleet" at the root,
// whose xpath is empty by design).
func validateRollbackTier(path string, attrs attributes.Attributes, errs []string) []string {
	rollback := attrs.Rollback

	if len(attrs.HealthChecks) > 0 && rollback != attributes.RollbackMagic {
		errs = append(errs, pathPrefix(path)+": health_checks requires rollback: magic")
	}

	if len(attrs.HealthChecksLocal) > 0 && (rollback == "" || rollback == attributes.RollbackOff) {
		errs = append(errs, pathPrefix(path)+": health_checks_local requires rollback: auto or magic")
	}

	if attrs.RollbackConfirmTimeout != 0 && rollback != attributes.RollbackMagic {
		errs = append(errs, pathPrefix(path)+": rollback_confirm_timeout requires rollback: magic")
	}

	// Spec 2 check budget: remote checks run inside the confirmation window with a
	// 30s guardian-internal timeout each, plus the 5s margin; the window must fit
	// them or panix could confirm past its own deadline.
	if rollback == attributes.RollbackMagic && len(attrs.HealthChecks) > 0 {
		budget := time.Duration(len(attrs.HealthChecks))*30*time.Second + 5*time.Second
		if budget > attrs.GetRollbackConfirmTimeout() {
			errs = append(errs, fmt.Sprintf(
				"%s: check budget (%d checks x 30s + 5s margin = %s) exceeds rollback_confirm_timeout (%s); raise rollback_confirm_timeout or remove checks",
				pathPrefix(path), len(attrs.HealthChecks), budget, attrs.GetRollbackConfirmTimeout()),
			)
		}
	}

	return errs
}

// pathPrefix formats an error location: an empty xpath (the fleet root) reads
// as "fleet".
func pathPrefix(path string) string {
	if path == "" {
		return "fleet"
	}

	return path
}
