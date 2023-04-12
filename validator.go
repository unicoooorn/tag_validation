package validation

import (
	"github.com/pkg/errors"
	"reflect"
	"strconv"
	"strings"
)

var ErrNotStruct = errors.New("wrong argument given, should be a struct")
var ErrInvalidValidatorSyntax = errors.New("invalid validator syntax")
var ErrValidateForUnexportedFields = errors.New("validation for unexported field is not allowed")

type ValidationError struct {
	Err error
}

func (ve ValidationError) Error() string {
	return ve.Err.Error()
}

type ValidationErrors []ValidationError

func (vs ValidationErrors) Error() string {
	res := ""
	for _, v := range vs {
		res = res + v.Err.Error()
	}
	return res
}

func Validate(v any) error {
	var vs ValidationErrors
	vType := reflect.TypeOf(v)
	vValue := reflect.ValueOf(v)
	validators := make(map[string]func(reflect.Value, string) (bool, error))
	if vType.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	validators["len"] = validateLen
	validators["in"] = validateIn
	validators["min"] = validateMin
	validators["max"] = validateMax
	validators["between"] = validateBetween

	for i := 0; i < vType.NumField(); i++ {
		curField := vType.Field(i)
		tagValue, ok := curField.Tag.Lookup("validate")
		if !ok {
			continue
		} else if !curField.IsExported() {
			vs = append(vs, ValidationError{ErrValidateForUnexportedFields})
			continue
		}
		rule := strings.Split(tagValue, ":")
		if len(rule) != 2 {
			vs = append(vs, ValidationError{ErrInvalidValidatorSyntax})
			continue
		}
		validator, ok := validators[rule[0]]
		if !ok {
			vs = append(vs, ValidationError{errors.New("Unexpected validator option")})
			continue
		}
		if ok, err := validator(vValue.Field(i), rule[1]); !ok {
			if validationErr, isValidationErr := err.(ValidationError); !isValidationErr {
				return err
			} else {
				vs = append(vs, validationErr)
				// изначально было вот так:
				// vs = append(vs, ValidationError{fmt.Errorf("\"%s\" field validation failed: %w", curField.Name, validationErr)})
				// но некоторые тесты требуют жёсткого совпадения текста ошибок: оборачивать их не получается
			}
		}
	}
	if len(vs) == 0 {
		return nil
	} else {
		return vs
	}
}

func validateLen(v reflect.Value, value string) (bool, error) {
	expected, err := strconv.Atoi(value)
	if err != nil {
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
	switch v.Interface().(type) {
	case string:
		if len(v.String()) != expected {
			return false, ValidationError{errors.New("lengths don't match")}
		}
		return true, nil
	case []string:
		var slice []string
		var ok bool
		if slice, ok = v.Interface().([]string); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if len(elem) != expected {
				return false, ValidationError{errors.Errorf("The string on position %d is shorter than allowed", i)}
			}
		}
		return true, nil
	default:
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
}

func validateIn(v reflect.Value, value string) (bool, error) {
	if len(value) == 0 {
		return false, ValidationError{errors.New("Field value isn't allowed")}
	}
	tokens := strings.Split(value, ",")
	tokensSet := make(map[string]struct{})
	for _, elem := range tokens {
		tokensSet[elem] = struct{}{}
	}
	switch v.Interface().(type) {
	case string:
		if _, ok := tokensSet[v.String()]; ok {
			return true, nil
		}
		return false, ValidationError{errors.New("Field value isn't allowed")}
	case int:
		for key := range tokensSet {
			val, err := strconv.Atoi(key)
			if err != nil {
				return false, ValidationError{ErrInvalidValidatorSyntax}
			}
			if int64(val) == v.Int() {
				return true, nil
			}
		}
		return false, ValidationError{errors.New("Field value isn't allowed")}
	case []string:
		var slice []string
		var ok bool
		if slice, ok = v.Interface().([]string); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if _, ok := tokensSet[elem]; !ok {
				return false, ValidationError{errors.Errorf("The string on position %d is not allowed", i)}
			}
		}
		return true, nil
	case []int:
		tokensSetInt := make(map[int]struct{})
		for elem := range tokensSet {
			elemInt, err := strconv.Atoi(elem)
			if err != nil {
				return false, ValidationError{ErrInvalidValidatorSyntax}
			}
			tokensSetInt[elemInt] = struct{}{}

		}
		var slice []int
		var ok bool
		if slice, ok = v.Interface().([]int); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if _, ok := tokensSetInt[elem]; !ok {
				return false, ValidationError{errors.Errorf("The integer on position %d is less than allowed", i)}
			}
		}
		return true, nil
	default:
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
}

func validateMin(v reflect.Value, value string) (bool, error) {
	min, err := strconv.Atoi(value)
	if err != nil {
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
	switch v.Interface().(type) {
	case string:
		if len(v.String()) >= min {
			return true, nil
		} else {
			return false, ValidationError{errors.New("String length is less than allowed")}
		}
	case int:
		if v.Int() >= int64(min) {
			return true, nil
		} else {
			return false, ValidationError{errors.New("Integer is less than allowed")}
		}
	case []int:
		var slice []int
		var ok bool
		if slice, ok = v.Interface().([]int); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if elem < min {
				return false, ValidationError{errors.Errorf("The integer on position %d is less than allowed", i)}
			}
		}
		return true, nil
	case []string:
		var slice []string
		var ok bool
		if slice, ok = v.Interface().([]string); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if len(elem) < min {
				return false, ValidationError{errors.Errorf("The string on position %d is shorter than allowed", i)}
			}
		}
		return true, nil
	default:
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
}

func validateBetween(v reflect.Value, value string) (bool, error) {
	limits := strings.Split(value, ",")
	min, err := strconv.Atoi(limits[0])
	max, err := strconv.Atoi(limits[1])
	if err != nil {
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
	switch v.Interface().(type) {
	case string:
		if min <= len(v.String()) && len(v.String()) <= max {
			return true, nil
		} else {
			return false, ValidationError{errors.New("String length is not allowed")}
		}
	case int:
		if int64(min) <= v.Int() && v.Int() <= int64(max) {
			return true, nil
		} else {
			return false, ValidationError{errors.New("Integer is more than allowed")}
		}
	case []int:
		var slice []int
		var ok bool
		if slice, ok = v.Interface().([]int); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if elem > max || elem < min {
				return false, ValidationError{errors.Errorf("The integer on position %d is more than allowed", i)}
			}
		}
		return true, nil
	case []string:
		var slice []string
		var ok bool
		if slice, ok = v.Interface().([]string); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if len(elem) > max || len(elem) < min {
				return false, ValidationError{errors.Errorf("The string on position %d is longer than allowed", i)}
			}
		}
		return true, nil
	default:
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
}

func validateMax(v reflect.Value, value string) (bool, error) {
	max, err := strconv.Atoi(value)
	if err != nil {
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
	switch v.Interface().(type) {
	case string:
		if len(v.String()) <= max {
			return true, nil
		} else {
			return false, ValidationError{errors.New("String length is more than allowed")}
		}
	case int:
		if v.Int() <= int64(max) {
			return true, nil
		} else {
			return false, ValidationError{errors.New("Integer is more than allowed")}
		}
	case []int:
		var slice []int
		var ok bool
		if slice, ok = v.Interface().([]int); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if elem > max {
				return false, ValidationError{errors.Errorf("The integer on position %d is more than allowed", i)}
			}
		}
		return true, nil
	case []string:
		var slice []string
		var ok bool
		if slice, ok = v.Interface().([]string); !ok {
			return false, ValidationError{ErrInvalidValidatorSyntax}
		}
		for i, elem := range slice {
			if len(elem) > max {
				return false, ValidationError{errors.Errorf("The string on position %d is longer than allowed", i)}
			}
		}
		return true, nil
	default:
		return false, ValidationError{ErrInvalidValidatorSyntax}
	}
}
