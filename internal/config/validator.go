package config

import (
	"path/filepath"

	"github.com/go-playground/validator/v10"
)

func ValidateConfig(conf *Config) error {
	v := validator.New()

	v.RegisterValidation("abspath", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()

		return filepath.IsAbs(val)
	})

	return v.Struct(conf)
}
