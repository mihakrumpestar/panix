package config

import (
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

func (conf *Config) ValidateStructTags() error {
	validate := validator.New()

	validate.RegisterValidation("abspath", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()

		return filepath.IsAbs(val)
	})

	return errors.Wrap(validate.Struct(conf), "validation failed")
}
