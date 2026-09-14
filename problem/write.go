package problem

import (
	"fmt"
	"net/http"

	"github.com/uchaloop/httpx/response"
)

// Write writes p as application/problem+json through net/http.
//
// The complete body is marshaled before headers are committed. Write returns
// an error without changing the response when the problem status is invalid or
// JSON marshaling fails.
func Write(w http.ResponseWriter, p Problem) error {
	if p.Status < http.StatusBadRequest || p.Status > 599 {
		return fmt.Errorf("status must be between 400 and 599, got %d", p.Status)
	}

	return response.Write(w, p.Response())
}
