package problem_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/uchaloop/httpx/problem"
)

func TestMakeProblemAppliesMapperDefaults(t *testing.T) {
	t.Parallel()

	got := problem.MakeProblem(
		httptest.NewRequest(http.MethodGet, "/articles/42?draft=true", nil),
		problem.Template{Status: http.StatusNotFound},
	)

	want := problem.Problem{Type: "about:blank", Title: "Not Found", Status: http.StatusNotFound, Instance: "/articles/42"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problem = %+v, want %+v", got, want)
	}

	answer := got.Response()
	if answer.Status != http.StatusNotFound || answer.ContentType != problem.ContentType || !reflect.DeepEqual(answer.Body, got) {
		t.Fatalf("response = %+v, want the problem as %s", answer, problem.ContentType)
	}
}

// Every adapter reports undecodable values with TypeParam, so its text is part
// of the public contract.
func TestTypeParamDescribesTargetType(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		target  reflect.Type
		message string
	}{
		{target: reflect.TypeFor[int64](), message: "id must be an integer"},
		{target: reflect.TypeFor[*uint](), message: "id must be a non-negative integer"},
		{target: reflect.TypeFor[float64](), message: "id must be a number"},
		{target: reflect.TypeFor[bool](), message: "id must be a boolean"},
		{target: reflect.TypeFor[string](), message: "id must be a string"},
		{target: reflect.TypeFor[[]int](), message: "id must be an array"},
		{target: reflect.TypeFor[struct{}](), message: "id must be an object"},
		{target: reflect.TypeFor[time.Time](), message: "id has an invalid format"},
	} {
		want := problem.InvalidParam{Path: "id", Code: "type", Message: test.message}
		if got := problem.TypeParam("id", test.target); got != want {
			t.Errorf("TypeParam(%s) = %+v, want %+v", test.target, got, want)
		}
	}
}
