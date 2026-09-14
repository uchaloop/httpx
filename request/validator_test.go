package request_test

import (
	"errors"
	"testing"

	"github.com/uchaloop/httpx/request"
)

type validatorSpy struct {
	value  any
	err    error
	called int
}

func (v *validatorSpy) Validate(value any) error {
	v.value = value
	v.called++

	return v.err
}

func TestDecodeAndValidateJSON(t *testing.T) {
	t.Parallel()

	validator := &validatorSpy{}

	got, err := request.DecodeAndValidateJSON[warehouseInput](
		makeJSONRequest(t, `{"name":"Minsk","id":42}`, "application/json"),
		validator,
	)
	if err != nil {
		t.Fatalf("DecodeAndValidateJSON: %v", err)
	}

	if got.Name != "Minsk" || got.ID != 42 {
		t.Errorf("decoded value = %+v, want Minsk and 42", got)
	}

	if validator.called != 1 {
		t.Fatalf("validator calls = %d, want 1", validator.called)
	}

	validated, ok := validator.value.(*warehouseInput)
	if !ok {
		t.Fatalf("validated value type = %T, want *warehouseInput", validator.value)
	}

	if validated.Name != "Minsk" || validated.ID != 42 {
		t.Errorf("validated value = %+v, want decoded input", validated)
	}
}

func TestDecodeAndValidateJSONPreservesValidationError(t *testing.T) {
	t.Parallel()

	validationErr := errors.New("name is required")
	validator := &validatorSpy{err: validationErr}

	_, err := request.DecodeAndValidateJSON[warehouseInput](
		makeJSONRequest(t, `{"name":"Minsk"}`, "application/json"),
		validator,
	)
	if !errors.Is(err, validationErr) {
		t.Fatalf("error = %v, want wrapped validation error", err)
	}
}

func TestDecodeAndValidateJSONDoesNotValidateFailedDecode(t *testing.T) {
	t.Parallel()

	validator := &validatorSpy{}

	_, err := request.DecodeAndValidateJSON[warehouseInput](
		makeJSONRequest(t, `{"id":"wrong"}`, "application/json"),
		validator,
	)

	assertDecodeErrorKind(t, err, request.DecodeErrorWrongType)
	if validator.called != 0 {
		t.Errorf("validator calls = %d, want 0", validator.called)
	}
}

func TestDecodeAndValidateJSONRejectsNilValidator(t *testing.T) {
	t.Parallel()

	_, err := request.DecodeAndValidateJSON[warehouseInput](
		makeJSONRequest(t, `{}`, "application/json"),
		nil,
	)

	if err == nil {
		t.Fatal("DecodeAndValidateJSON succeeded, want nil validator error")
	}
}
