package problem_test

import (
	"net/http"
	"testing"

	"github.com/uchaloop/httpx/apitest"
	"github.com/uchaloop/httpx/problem"
)

// A problem may call for a header RFC 9110 requires, such as WWW-Authenticate
// on 401. The header is sent with the problem but stays out of its document.
func TestProblemHeaderIsSentOutsideTheDocument(t *testing.T) {
	t.Parallel()

	unauthorized := problem.MakeProblem(
		&http.Request{},
		problem.Template{Status: http.StatusUnauthorized, Header: http.Header{"WWW-Authenticate": {`Bearer realm="api"`}}},
	)

	apitest.Make(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := problem.Write(w, unauthorized); err != nil {
			t.Errorf("write problem: %v", err)
		}
	})).Get("/").Do().
		Status(http.StatusUnauthorized).
		Header("WWW-Authenticate", `Bearer realm="api"`).
		JSONEqual(map[string]any{"type": "about:blank", "title": "Unauthorized", "status": 401})
}
