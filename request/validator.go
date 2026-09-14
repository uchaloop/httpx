package request

import (
	"fmt"
	"net/http"
)

// Validator describes the validation capability consumed by request helpers.
// Concrete validator adapters implement this interface outside the request
// package.
type Validator interface {
	Validate(any) error
}

// DecodeAndValidateJSON decodes one JSON document and then validates the
// resulting *T. Decode errors and validation errors preserve their concrete
// types through errors.Unwrap.
func DecodeAndValidateJSON[T any](
	r *http.Request,
	v Validator,
	opts ...JSONOption,
) (T, error) {
	value, err := DecodeJSON[T](r, opts...)
	if err != nil {
		return value, err
	}

	if v == nil {
		return value, fmt.Errorf("validator is nil")
	}

	if err := v.Validate(&value); err != nil {
		return value, fmt.Errorf("validate decoded JSON: %w", err)
	}

	return value, nil
}
