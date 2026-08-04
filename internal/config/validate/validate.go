package validate

import (
	"fmt"
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

func ValidateStructTags[T any](s T, f *fleet.Fleet, flags flags.ValidateFlags, flakeValidationTimeout time.Duration) error { //nolint:varnamelen
	validate := validator.New(validator.WithRequiredStructEnabled(), validator.WithPrivateFieldValidation(), validator.WithRequiredStructEnabled())

	registerPathValidators(validate)

	err := validate.Struct(s)
	if err != nil {
		return errors.New(humanizeValidationErrors(err))
	}

	err = validatePaths(f, flags)
	if err != nil {
		return err
	}

	if flags.Validate.Flakes {
		err = validateFlakes(f, flakeValidationTimeout)
		if err != nil {
			return errors.Wrap(err, "invalid flakes configuration")
		}
	}

	err = validateBuildModes(f)
	if err != nil {
		return errors.Wrap(err, "invalid build mode configuration")
	}

	err = validateOutputTypes(f)
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

func validateOutputTypes(fleetConfig *fleet.Fleet) error {
	var errs []string

	knownTypes := installablepkg.KnownOutputTypes()

	knownTypeStrs := make([]string, len(knownTypes))
	for i, t := range knownTypes {
		knownTypeStrs[i] = t.String()
	}

	for _, flakePair := range fleetConfig.Flakes.Pairs() {
		flakePair.Value.Installables.ForEach(func(typeKey string, attrMap *atomicorderedmap.AtomicOrderedMap[string, *installablepkg.Installable]) bool {
			if attrMap == nil {
				return true
			}

			attrMap.ForEach(func(nameKey string, installable *installablepkg.Installable) bool {
				if installable == nil {
					return true
				}

				if !installablepkg.FlakeOutputType(typeKey).IsKnown() {
					errs = append(errs, fmt.Sprintf("%s: unknown output type '%s', known types: %s",
						installable.Xpath.String(), typeKey, strings.Join(knownTypeStrs, ", ")))
				}

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
