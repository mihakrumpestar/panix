package validate

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/pkg/errors"
	"github.com/stoewer/go-strcase"
)

func ValidateStructTags[T any](s T, f *fleet.Fleet, flags flags.ValidateFlags) error { //nolint:varnamelen
	validate := validator.New(validator.WithPrivateFieldValidation(), validator.WithRequiredStructEnabled())

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
		err = validateFlakes(f)
		if err != nil {
			return errors.Wrap(err, "invalid flakes configuration")
		}
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
	"Config": true, "Flake": true, "Configuration": true, "Machine": true, "Attributes": true, "Values": true,
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
