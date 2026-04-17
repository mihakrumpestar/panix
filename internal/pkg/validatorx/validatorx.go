package validatorx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

func ValidateStructTags[T any](v T) error {
	validate := validator.New(validator.WithPrivateFieldValidation(), validator.WithRequiredStructEnabled())

	err := validate.RegisterValidation("abspath", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()

		return filepath.IsAbs(val)
	})
	if err != nil {
		return errors.Wrap(err, "failed to register abspath validation")
	}

	err = validate.RegisterValidation("pathexists", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if val == "" {
			return true
		}

		_, err := os.Stat(val)

		return err == nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to register pathexists validation")
	}

	err = validate.Struct(v)
	if err != nil {
		return errors.New(humanizeValidationErrors(err))
	}

	return nil
}

// humanizeValidationErrors converts validator.ValidationErrors into a user-friendly
// multi-line error message with YAML-style paths and human-readable descriptions.
func humanizeValidationErrors(err error) string {
	var ve validator.ValidationErrors
	if !extractValidationErrors(err, &ve) {
		return err.Error()
	}

	seen := make(map[string]bool, len(ve))
	var b strings.Builder

	b.WriteString("configuration validation errors:\n")

	for _, fe := range ve {
		path := humanizePath(fe.Namespace())
		msg := humanizeTagMessage(fe)

		key := fmt.Sprintf("%s: %s", path, msg)
		if !seen[key] {
			seen[key] = true
			b.WriteString(fmt.Sprintf("  - %s\n", key))
		}
	}

	return b.String()
}

// extractValidationErrors unwraps err to find validator.ValidationErrors.
func extractValidationErrors(err error, ve *validator.ValidationErrors) bool {
	for {
		switch e := err.(type) {
		case validator.ValidationErrors:
			*ve = e
			return true
		case interface{ Unwrap() error }:
			err = e.Unwrap()
			if err == nil {
				return false
			}
		default:
			return false
		}
	}
}

// humanizePath converts a validator namespace like
// "Config.Fleet.Flakes.Values[*].Configurations.Values[*].Machines.Values[*].Attributes.Bootstrap.DiskEncryptionKeys[0].LocalPath"
// into a readable dotted path like
// "fleet.flakes.configurations.machines.bootstrap.disk_encryption_keys[0].local_path"
func humanizePath(namespace string) string {
	parts := strings.Split(namespace, ".")
	var result []string

	for _, part := range parts {
		if part == "Config" || part == "" {
			continue
		}

		if isTypeNode(part) {
			continue
		}

		result = append(result, camelToSnake(part))
	}

	return strings.Join(result, ".")
}

// isTypeNode returns true for Go type names that appear as intermediate namespace nodes
// but don't represent actual config keys.
func isTypeNode(part string) bool {
	switch part {
	case "Flake", "Configuration", "Machine", "Attributes", "Values":
		return true
	}

	return false
}

// camelToSnake converts CamelCase to snake_case, preserving array indices.
func camelToSnake(s string) string {
	suffix := ""
	if idx := strings.Index(s, "["); idx >= 0 {
		suffix = s[idx:]
		s = s[:idx]
	}

	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			nextLower := i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
			prevLower := s[i-1] >= 'a' && s[i-1] <= 'z'
			if nextLower || prevLower {
				b.WriteByte('_')
			}
		}

		if r >= 'A' && r <= 'Z' {
			r |= 0x20
		}

		b.WriteRune(r)
	}

	return b.String() + suffix
}

// humanizeTagMessage returns a human-readable message for a failed validation tag.
func humanizeTagMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "pathexists":
		return fmt.Sprintf("path does not exist: %v", fe.Value())
	case "filepath":
		return fmt.Sprintf("invalid file path: %v", fe.Value())
	case "abspath":
		return fmt.Sprintf("must be an absolute path, got: %v", fe.Value())
	case "min":
		return fmt.Sprintf("must have at least %s items, but has %v", fe.Param(), fe.Value())
	case "url":
		return fmt.Sprintf("must be a valid URL, got: %v", fe.Value())
	case "uri":
		return fmt.Sprintf("must be a valid URI, got: %v", fe.Value())
	case "oneof":
		return fmt.Sprintf("must be one of [%s], got: %v", fe.Param(), fe.Value())
	case "dive":
		return "contains invalid elements"
	default:
		return fmt.Sprintf("failed validation '%s' (value: %v)", fe.Tag(), fe.Value())
	}
}
