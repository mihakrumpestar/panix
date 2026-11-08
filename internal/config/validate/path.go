package validate

import (
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/pkg/urlx"
	"github.com/pkg/errors"
)

var (
	ErrPathValidation = errors.New("path validation errors")
)

func registerPathValidators(validate *validator.Validate) {
	err := validate.RegisterValidation("abspath", func(fl validator.FieldLevel) bool {
		return filepath.IsAbs(fl.Field().String())
	})
	if err != nil {
		panic(errors.Wrap(err, "failed to register abspath validation"))
	}

	// dir_shape: theoretical check only, the path need not exist on disk; rejects empty and NUL-byte paths.
	err = validate.RegisterValidation("dir_shape", func(fl validator.FieldLevel) bool {
		p := strings.TrimSpace(fl.Field().String())

		return p != "" && !strings.ContainsRune(p, 0)
	})
	if err != nil {
		panic(errors.Wrap(err, "failed to register dir_shape validation"))
	}

	// Shape check only: a local file need not exist at config load time.
	err = validate.RegisterValidation("url_or_file", func(fl validator.FieldLevel) bool {
		return urlx.ValidateSource(fl.Field().String()) == nil
	})
	if err != nil {
		panic(errors.Wrap(err, "failed to register url_or_file validation"))
	}

	// nonempty rejects an explicitly empty command: nil means "use the built-in
	// one", while [] would leave nothing to run.
	err = validate.RegisterValidation("nonempty", func(fl validator.FieldLevel) bool {
		return fl.Field().IsNil() || fl.Field().Len() > 0
	})
	if err != nil {
		panic(errors.Wrap(err, "failed to register nonempty validation"))
	}
}

func validatePaths(fleetConfig *fleet.Fleet, flags flags.ValidateFlags) error {
	var errs []string

	for _, fleetLeaf := range fleetConfig.AllMachines() {
		machine := fleetLeaf.Machine

		err := machine.ValidateSecretsPaths()
		if err != nil {
			errs = append(errs, err.Error())
		}

		if flags.Validate.BootstrapSecrets {
			err = machine.ValidateBootstrapSecretsPaths()
			if err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	errs = append(errs, validateHardwareConfigPathUniqueness(fleetConfig)...)

	if len(errs) > 0 {
		return errors.Wrapf(ErrPathValidation, "\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// validateHardwareConfigPathUniqueness rejects two machines writing the same
// hardware config: the file describes one machine's hardware, so a shared path
// is always a misconfiguration, and generation would silently overwrite it.
// Paths are compared in absolute form so two spellings of one file collide.
func validateHardwareConfigPathUniqueness(fleetConfig *fleet.Fleet) []string {
	var errs []string

	owner := map[string]string{}

	for _, fleetLeaf := range fleetConfig.AllMachines() {
		machine := fleetLeaf.Machine

		path := machine.HardwareConfig.Path
		if path == "" || !fleetLeaf.Installable.Preset.BootstrapsNixOS() {
			continue
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			errs = append(errs, machine.Xpath.String()+": invalid hardware_config.path "+path)

			continue
		}

		if other, taken := owner[absPath]; taken {
			errs = append(errs, other+" and "+machine.Xpath.String()+" both write hardware_config.path "+path)

			continue
		}

		owner[absPath] = machine.Xpath.String()
	}

	return errs
}
