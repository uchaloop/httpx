package problem_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uchaloop/httpx/apitest"
	"github.com/uchaloop/httpx/problem"
)

func TestWriteProblemDetails(t *testing.T) {
	t.Parallel()

	apitest.Make(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := problem.Write(w, problem.Problem{
			Type:     "https://example.test/problems/validation",
			Title:    "Request validation failed",
			Status:   http.StatusUnprocessableEntity,
			Instance: "/warehouses",
			Code:     "validation_error",
			Errors: []problem.InvalidParam{{
				Path:    "name",
				Code:    "required",
				Message: "Field is required",
			}},
		}); err != nil {
			t.Errorf("write problem: %v", err)
		}
	})).Get("/warehouses").Do().
		Status(http.StatusUnprocessableEntity).
		Header("Content-Type", problem.ContentType).
		JSONEqual(map[string]any{
			"type":     "https://example.test/problems/validation",
			"title":    "Request validation failed",
			"status":   422,
			"instance": "/warehouses",
			"code":     "validation_error",
			"errors": []any{
				map[string]any{"path": "name", "code": "required", "message": "Field is required"},
			},
		})
}

func TestWriteRejectsInvalidStatusBeforeCommit(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	if err := problem.Write(recorder, problem.Problem{Status: http.StatusOK}); err == nil {
		t.Fatal("Write succeeded, want invalid status error")
	}

	if recorder.Code != http.StatusOK {
		t.Errorf("recorder status = %d, want untouched default 200", recorder.Code)
	}

	if recorder.Body.Len() != 0 {
		t.Errorf("recorder body = %q, want empty", recorder.Body.String())
	}
}
