package validate

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/nix"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	installablepkg "github.com/mihakrumpestar/panix/internal/config/tree/installable"
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
	case "oneof":
		return fmt.Sprintf("must be one of [%s], got: %v", fieldError.Param(), fieldError.Value())
	case "dive":
		return "contains invalid elements"
	default:
		return fmt.Sprintf("failed validation '%s' (value: %v)", fieldError.Tag(), fieldError.Value())
	}
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

	machineCount := 0
	firstMachineIsLocal := false

	for i, machinePair := range out.Machines.Pairs() {
		machineCount++

		if i == 0 && machinePair.Value != nil && machinePair.Value.SSH.IsLocal() {
			firstMachineIsLocal = true
		}
	}

	if machineCount == 0 {
		errs = append(errs, outPath+": remote mode requires at least 1 machine")
	}

	if firstMachineIsLocal {
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
// set_profile: true requires a profile_path.
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

		return true
	})

	return errs
}
