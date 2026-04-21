package validate

import (
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
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
}

func validatePaths(f *fleet.Fleet, flags flags.ValidateFlags) error {
	var errs []string

	for _, fleetLeaf := range f.AllMachines() {
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

	if len(errs) > 0 {
		return errors.Wrapf(ErrPathValidation, "\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
