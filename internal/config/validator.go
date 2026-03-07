package config

import (
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

func (c *Config) ValidateStructTags() error {
	validate := validator.New()

	err := validate.RegisterValidation("abspath", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()

		return filepath.IsAbs(val)
	})
	if err != nil {
		return errors.Wrap(err, "failed to register abspath validation")
	}

	return errors.Wrap(validate.Struct(c), "validation failed")
}
