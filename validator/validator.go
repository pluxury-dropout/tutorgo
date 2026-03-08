package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// to get fields' names from json (not from go structs)
func init() {
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}
func Validate(s any) map[string]string {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	errors := make(map[string]string)
	for _, err := range err.(validator.ValidationErrors) {
		field := err.Field()
		errors[field] = formatError(err)
	}
	return errors
}

func formatError(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", strings.ToLower(err.Field()))
	case "email":
		return "invalid email format"
	case "min":
		return fmt.Sprintf("minimum length is %s", err.Param())
	case "max":
		return fmt.Sprintf("maximum length is %s", err.Param())
	case "gt":
		return fmt.Sprintf("must be greater than %s", err.Param())
	default:
		return fmt.Sprintf("%s is invalid", strings.ToLower(err.Field()))
	}
}
