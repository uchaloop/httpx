package problem_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uchaloop/httpx/problem"
)

var errWarehouseNotFound = errors.New("warehouse not found")

type validationError struct {
	field string
}

func (e *validationError) Error() string {
	return "validation failed for " + e.field
}

func TestMapperUsesFirstMatchingRule(t *testing.T) {
	t.Parallel()

	mapper, err := problem.MakeMapper(
		problem.WhenIs(errWarehouseNotFound, problem.Template{
			Status: http.StatusNotFound,
			Code:   "specific_not_found",
		}),
		problem.When(func(err error) (problem.Template, bool) {
			return problem.Template{
				Status: http.StatusBadRequest,
				Code:   "generic_match",
			}, errors.Is(err, errWarehouseNotFound)
		}),
	)
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	// Defaults and the instance are covered by TestMakeProblemAppliesMapperDefaults.
	got := mapper.Map(
		httptest.NewRequest(http.MethodGet, "/warehouses/42", nil),
		fmt.Errorf("fetch warehouse: %w", errWarehouseNotFound),
	)
	if got.Status != http.StatusNotFound || got.Code != "specific_not_found" {
		t.Errorf("problem = %+v, want the first rule's 404 specific_not_found", got)
	}
}

func TestMapperMatchesTypedErrorAndBuildsValidationProblem(t *testing.T) {
	t.Parallel()

	mapper, err := problem.MakeMapper(
		problem.WhenAs[*validationError](func(err *validationError) problem.Template {
			return problem.Template{
				Type:   "https://example.test/problems/validation",
				Title:  "Request validation failed",
				Status: http.StatusUnprocessableEntity,
				Code:   "validation_error",
				Errors: []problem.InvalidParam{{
					Path:    err.field,
					Code:    "required",
					Message: "Field is required",
				}},
			}
		}),
	)
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	got := mapper.Map(
		httptest.NewRequest(http.MethodPost, "/warehouses", nil),
		fmt.Errorf("decode input: %w", &validationError{field: "name"}),
	)

	if got.Status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", got.Status)
	}

	if got.Code != "validation_error" {
		t.Errorf("code = %q, want validation_error", got.Code)
	}

	if len(got.Errors) != 1 {
		t.Fatalf("errors length = %d, want 1", len(got.Errors))
	}

	if got.Errors[0].Path != "name" {
		t.Errorf("error path = %q, want name", got.Errors[0].Path)
	}
}

func TestMapperHidesUnknownError(t *testing.T) {
	t.Parallel()

	mapper, err := problem.MakeMapper()
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	got := mapper.Map(
		httptest.NewRequest(http.MethodGet, "/warehouses", nil),
		errors.New("database password is secret"),
	)

	if got.Status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", got.Status)
	}

	if got.Code != "internal_error" {
		t.Errorf("code = %q, want internal_error", got.Code)
	}

	if got.Title != http.StatusText(http.StatusInternalServerError) {
		t.Errorf("title = %q, want generic internal title", got.Title)
	}

	if len(got.Detail) > 0 {
		t.Errorf("detail = %q, want empty", got.Detail)
	}
}

func TestMapKnownDistinguishesExplicitRuleFromFallback(t *testing.T) {
	t.Parallel()

	mapper, err := problem.MakeMapper(problem.WhenIs(errWarehouseNotFound, problem.Template{
		Status: http.StatusNotFound,
		Code:   "warehouse_not_found",
	}))
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	if mapped, matched := mapper.MapKnown(nil, errWarehouseNotFound); !matched || mapped.Status != http.StatusNotFound {
		t.Fatalf("known mapping = (%+v, %t), want matched 404", mapped, matched)
	}

	if mapped, matched := mapper.MapKnown(nil, errors.New("unknown")); matched || mapped.Status != 0 {
		t.Fatalf("unknown mapping = (%+v, %t), want zero and unmatched", mapped, matched)
	}
}

func TestMapperFallsBackWhenDynamicRuleReturnsInvalidTemplate(t *testing.T) {
	t.Parallel()

	mapper, err := problem.MakeMapper(problem.When(func(error) (problem.Template, bool) {
		return problem.Template{Status: http.StatusOK, Detail: "must not leak"}, true
	}))
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	got := mapper.Map(nil, errors.New("source error"))
	if got.Status != http.StatusInternalServerError || len(got.Detail) > 0 {
		t.Fatalf("problem = %+v, want safe internal fallback", got)
	}
}

func TestMakeMapperRejectsInvalidRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rule problem.Rule
	}{
		{name: "nil target", rule: problem.WhenIs(nil, problem.Template{Status: http.StatusNotFound})},
		{name: "invalid status", rule: problem.WhenIs(errWarehouseNotFound, problem.Template{Status: http.StatusOK})},
		{name: "nil custom matcher", rule: problem.When(nil)},
		{name: "zero rule", rule: problem.Rule{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := problem.MakeMapper(tt.rule); err == nil {
				t.Fatal("MakeMapper succeeded, want configuration error")
			}
		})
	}
}

func TestProblemOwnsCopyOfTemplateErrors(t *testing.T) {
	t.Parallel()

	params := []problem.InvalidParam{{Path: "name", Message: "required"}}
	mapper, err := problem.MakeMapper(problem.WhenIs(errWarehouseNotFound, problem.Template{
		Status: http.StatusBadRequest,
		Errors: params,
	}))
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	got := mapper.Map(nil, errWarehouseNotFound)
	params[0].Path = "changed"

	if got.Errors[0].Path != "name" {
		t.Errorf("mapped error path = %q, want independent copy", got.Errors[0].Path)
	}
}
