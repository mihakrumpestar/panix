package config

import (
	"path/filepath"
	"reflect"

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

	validate.RegisterStructValidation(validateOrderedMap[string, *Flake], OrderedMap[string, *Flake]{})
	validate.RegisterStructValidation(validateOrderedMap[string, *Configuration], OrderedMap[string, *Configuration]{})
	validate.RegisterStructValidation(validateOrderedMap[string, *Machine], OrderedMap[string, *Machine]{})

	err = validate.Struct(c)
	if err != nil {
		return errors.Wrap(err, "struct validation failed")
	}

	return nil
}

func validateOrderedMap[K comparable, V any](structLevel validator.StructLevel) {
	omapField := structLevel.Current().FieldByName("Omap")
	if !omapField.IsValid() || omapField.IsNil() {
		return
	}

	pairsSlice := getPairsSlice(omapField)
	if !pairsSlice.IsValid() {
		return
	}

	for i := range pairsSlice.Len() {
		if !validatePair(structLevel, pairsSlice.Index(i)) {
			return
		}
	}
}

func getPairsSlice(omapField reflect.Value) reflect.Value {
	pairsMethod := omapField.MethodByName("Pairs")
	if !pairsMethod.IsValid() {
		return reflect.Value{}
	}

	results := pairsMethod.Call(nil)
	if len(results) == 0 {
		return reflect.Value{}
	}

	pairsSlice := results[0]
	if pairsSlice.Kind() != reflect.Slice {
		return reflect.Value{}
	}

	return pairsSlice
}

func validatePair(structLevel validator.StructLevel, pair reflect.Value) bool {
	if pair.Kind() != reflect.Struct {
		return true
	}

	valueField := pair.FieldByName("Value")
	if !valueField.IsValid() {
		return true
	}

	val, ok := unwrapValue(valueField)
	if !ok {
		return true
	}

	if val.Kind() == reflect.Struct {
		err := structLevel.Validator().Struct(val.Interface())
		if err != nil {
			reportNestedErrors(structLevel, val, err)

			return false
		}
	}

	return true
}

func unwrapValue(val reflect.Value) (reflect.Value, bool) {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return reflect.Value{}, false
		}

		return val.Elem(), true
	}

	return val, true
}

func reportNestedErrors(structLevel validator.StructLevel, val reflect.Value, err error) {
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		structLevel.ReportError(val, val.Type().Name(), "", "valid", err.Error())

		return
	}

	for _, e := range validationErrs {
		fieldName := e.Field()
		tag := e.Tag()
		param := e.Param()

		fieldVal := val.FieldByName(fieldName)
		if fieldVal.IsValid() {
			structLevel.ReportError(fieldVal, fieldName, fieldName, tag, param)
		} else {
			structLevel.ReportError(val, fieldName, fieldName, tag, param)
		}
	}
}
