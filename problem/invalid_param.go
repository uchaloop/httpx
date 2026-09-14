package problem

import (
	"encoding"
	"fmt"
	"reflect"
)

// Messages follow the English translations of go-playground/validator: the
// field name comes first, so a constraint reads the same whichever layer
// rejects it.

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// TypeParam describes a request value that cannot be decoded into target, the
// type of the field it binds to. Its code is type: validation never sees such
// a value, so no validation tag names the failure. Adapters use it for their
// binding errors so every framework reports the same text.
func TypeParam(path string, target reflect.Type) InvalidParam {
	return InvalidParam{Path: path, Code: "type", Message: path + " " + typeRequirement(target)}
}

func typeRequirement(target reflect.Type) string {
	for target != nil && target.Kind() == reflect.Pointer {
		target = target.Elem()
	}

	if target == nil || reflect.PointerTo(target).Implements(textUnmarshalerType) {
		return "has an invalid format"
	}

	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "must be an integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "must be a non-negative integer"
	case reflect.Float32, reflect.Float64:
		return "must be a number"
	case reflect.Bool:
		return "must be a boolean"
	case reflect.String:
		return "must be a string"
	case reflect.Slice, reflect.Array:
		return "must be an array"
	case reflect.Map, reflect.Struct:
		return "must be an object"
	default:
		return "has an invalid format"
	}
}

// minParam and maxParam use the validator tag names and number translations.
func minParam(path string, limit uint64) InvalidParam {
	return InvalidParam{Path: path, Code: "min", Message: fmt.Sprintf("%s must be %d or greater", path, limit)}
}

func maxParam(path string, limit uint64) InvalidParam {
	return InvalidParam{Path: path, Code: "max", Message: fmt.Sprintf("%s must be %d or less", path, limit)}
}
