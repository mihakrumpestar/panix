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

// ValidateStructTags validates the whole configuration: the output-type
// checks first, then struct-tag validation over the whole configuration (via
// reflection on conf, which must be the *config.Config value), followed by
// the path, flake, and build-mode checks. The output-type checks run first
// because validateDeclaredPresets is the authoritative bootstrap_mode check
// (house-style messages naming the YAML keys): the schema's oneof tag on
// Preset.Bootstrap is also enforced by the struct-tag walk, but the
// type-level value is propagated to every installable of the type, so that
// walk would report the same declaration mistake once per installable with
// Go field paths. The declaration checks short-circuit first, so the
// tag-driven message can never surface for output_types. The fleet and its
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

	err := validateOutputTypes(fl, outputTypes)
	if err != nil {
		return errors.Wrap(err, "invalid output type configuration")
	}

	err = validate.Struct(conf)
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
	case "dir_shape":
		return fmt.Sprintf("must be a directory path, got: %v", fieldError.Value())
	case "uri|dir":
		return fmt.Sprintf("must be a valid URL or an existing local directory, got: %v", fieldError.Value())
	case "url":
		return fmt.Sprintf("must be a valid URL, got: %v", fieldError.Value())
	case "uri":
		return fmt.Sprintf("must be a valid URI, got: %v", fieldError.Value())
	case "url_or_file":
		return fmt.Sprintf("must be an http(s) URL or a local path (unsupported schemes are rejected), got: %v", fieldError.Value())
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
// supported modes, and set_profile: true requires a profile_path. A declared
// bootstrap_mode must name a BootstrapMode ('nixos' additionally requires
// system_level: true and the NixOS system toplevel build_path).
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
// declared in activation_supported_modes. bootstrap_mode must name a
// BootstrapMode; 'nixos' mirrors the built-in invariant (system-level and the
// NixOS system toplevel build path), 'nix-install' has no extra requirements.
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

		errs = append(errs, validateDeclaredBootstrapMode(typ, preset)...)

		// The mode semantic lists reference modes the activation script
		// understands; entries outside activation_supported_modes would
		// silently never match at runtime.
		errs = append(errs, validateModeSubset(typ, preset.NonMutatingModes, preset.ActivationModes, "activation_non_mutating_modes")...)
		errs = append(errs, validateModeSubset(typ, preset.ProfileSkipModes, preset.ActivationModes, "activation_profile_skip_modes")...)

		return true
	})

	return errs
}

// validateDeclaredBootstrapMode checks the declared bootstrap_mode of a
// custom output type, returning an error message per violated rule: the mode
// must name one of the BootstrapMode constants (the empty default disables
// bootstrap), 'nixos' mirrors the built-in NixOS invariant (system-level
// because kexec, disko and nixos-install need root, and the standard NixOS
// system toplevel build path), while 'nix-install' works on any type.
func validateDeclaredBootstrapMode(typ installablepkg.FlakeOutputType, preset installablepkg.Preset) []string {
	var errs []string

	switch preset.Bootstrap {
	case installablepkg.BootstrapNone, installablepkg.BootstrapNixOS, installablepkg.BootstrapNixInstall:
		// Valid bootstrap mode.
	default:
		return []string{fmt.Sprintf(
			"output_types: '%s' bootstrap_mode '%s' is not a valid bootstrap mode, must be one of '%s' or '%s' (or empty to disable bootstrap)",
			typ, preset.Bootstrap, installablepkg.BootstrapNixOS, installablepkg.BootstrapNixInstall,
		)}
	}

	if preset.Bootstrap != installablepkg.BootstrapNixOS {
		return errs
	}

	if !preset.IsSystemLevelValue() {
		errs = append(errs, fmt.Sprintf(
			"output_types: '%s' bootstrap_mode '%s' requires system_level: true (bootstrap needs root)",
			typ, installablepkg.BootstrapNixOS,
		))
	}

	// The NixOS bootstrap runs nixos-install on the built system closure and
	// resolves the disko script under the same output, so the output must be
	// a standard NixOS system evaluation.
	if preset.BuildPath != installablepkg.NixOSSystemBuildPath {
		errs = append(errs, fmt.Sprintf(
			"output_types: '%s' bootstrap_mode '%s' requires build_path '%s' "+
				"(the NixOS bootstrap runs nixos-install on the built system closure and resolves %s under the same output, "+
				"so the output must be a standard NixOS system evaluation)",
			typ, installablepkg.BootstrapNixOS, installablepkg.NixOSSystemBuildPath, installablepkg.NixOSDiskoScriptPath,
		))
	}

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
