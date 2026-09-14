package problem_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/uchaloop/httpx/problem"
)

func TestErrorRuleMapsTransportProblemAndPreservesCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("page size exceeds maximum")
	params := []problem.InvalidParam{{
		Path:    "size",
		Code:    "max",
		Message: "Value must be at most 100",
	}}

	mapper, err := problem.MakeMapper(problem.ErrorRule())
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	transportErr := problem.MakeInvalidRequest(http.StatusUnprocessableEntity, cause, params...)
	params[0].Path = "changed"

	if !errors.Is(transportErr, cause) {
		t.Fatalf("error = %v, want cause in chain", transportErr)
	}

	mapped := mapper.Map(nil, transportErr)
	if mapped.Status != http.StatusUnprocessableEntity || mapped.Code != "invalid_request" {
		t.Fatalf("problem = %+v, want invalid request 422", mapped)
	}

	if len(mapped.Errors) != 1 || mapped.Errors[0].Path != "size" {
		t.Fatalf("problem errors = %+v, want owned size error", mapped.Errors)
	}
}
