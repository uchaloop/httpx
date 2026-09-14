package problem_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/uchaloop/httpx/page"
	"github.com/uchaloop/httpx/problem"
	"github.com/uchaloop/httpx/request"
	"github.com/uchaloop/httpx/sortby"
)

func TestInputProblemRuleMapsHTTPXInputErrors(t *testing.T) {
	t.Parallel()

	makePageError := func(params page.Params) error {
		_, err := page.Make(params, page.Config{DefaultSize: 10, MaxSize: 100, MaxOffset: 10})

		return err
	}

	zero, tooLarge, third := uint64(0), uint64(101), uint64(3)
	makeSortError := func(expressions ...string) error {
		_, err := sortby.Parse(expressions, func(field string) (string, error) {
			if field != "id" {
				return "", errors.New("unsupported field")
			}

			return field, nil
		})

		return err
	}

	makeDecodeError := func(body string) error {
		_, err := request.DecodeJSON[struct {
			Title string `json:"title"`
		}](makeJSONRequest(t, body, "application/json"))

		return err
	}

	makeLimitedDecodeError := func(body string, limit int64) error {
		httpRequest := makeJSONRequest(t, body, "application/json")
		httpRequest.Body = http.MaxBytesReader(httptest.NewRecorder(), httpRequest.Body, limit)
		_, err := request.DecodeJSON[map[string]any](httpRequest)

		return err
	}

	makeMediaTypeError := func() error {
		_, err := request.DecodeJSON[map[string]any](makeJSONRequest(t, "title=HTTP", "text/plain"))

		return err
	}

	tests := []struct {
		name   string
		err    error
		status int
		code   string
		errors []problem.InvalidParam
		header http.Header
	}{
		{
			name: "unsupported media type", err: makeMediaTypeError(),
			status: http.StatusUnsupportedMediaType, header: http.Header{"Accept": {"application/json"}},
		},
		{
			name: "page", err: makePageError(page.Params{Number: &zero}),
			status: http.StatusUnprocessableEntity, code: "invalid_request",
			errors: []problem.InvalidParam{{Path: "page", Code: "min", Message: "page must be 1 or greater"}},
		},
		{
			name: "size", err: makePageError(page.Params{Size: &tooLarge}),
			status: http.StatusUnprocessableEntity, code: "invalid_request",
			errors: []problem.InvalidParam{{Path: "size", Code: "max", Message: "size must be 100 or less"}},
		},
		{
			name: "offset", err: makePageError(page.Params{Number: &third}),
			status: http.StatusUnprocessableEntity, code: "invalid_request",
			errors: []problem.InvalidParam{{Path: "page", Code: "max", Message: "page must be 2 or less"}},
		},
		{
			name: "sort field", err: makeSortError("id", "secret"),
			status: http.StatusUnprocessableEntity, code: "invalid_request",
			errors: []problem.InvalidParam{{Path: "sort.1", Code: "enum", Message: "sort[1] must be one of the allowed values"}},
		},
		{
			name: "sort direction", err: makeSortError("id:sideways"),
			status: http.StatusUnprocessableEntity, code: "invalid_request",
			errors: []problem.InvalidParam{{Path: "sort.0", Code: "enum", Message: "sort[0] must be one of the allowed values"}},
		},
		{
			name: "sort expression", err: makeSortError("id:asc:desc"),
			status: http.StatusUnprocessableEntity, code: "invalid_request",
			errors: []problem.InvalidParam{{Path: "sort.0", Code: "enum", Message: "sort[0] must be one of the allowed values"}},
		},
		{
			name: "malformed JSON", err: makeDecodeError(`{"title":`),
			status: http.StatusBadRequest, code: "invalid_json",
		},
		{
			name: "JSON type", err: makeDecodeError(`{"title":5}`),
			status: http.StatusBadRequest, code: "invalid_json",
			errors: []problem.InvalidParam{{Path: "title", Code: "type", Message: "title must be a string"}},
		},
		{
			name: "body too large", err: makeLimitedDecodeError(`{"title":"too long"}`, 4),
			status: http.StatusRequestEntityTooLarge,
		},
	}

	mapper, err := problem.MakeMapper(problem.InputProblemRule())
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mapped := mapper.Map(nil, test.err)
			if mapped.Status != test.status || mapped.Code != test.code {
				t.Fatalf("problem = %+v, want status %d code %q", mapped, test.status, test.code)
			}

			if !slices.Equal(mapped.Errors, test.errors) {
				t.Fatalf("problem errors = %+v, want %+v", mapped.Errors, test.errors)
			}

			if !reflect.DeepEqual(mapped.Header, test.header) {
				t.Fatalf("problem header = %v, want %v", mapped.Header, test.header)
			}
		})
	}
}

func TestInputProblemRuleDoesNotExposeInvalidPageConfiguration(t *testing.T) {
	t.Parallel()

	_, configErr := page.Make(page.Params{}, page.Config{})
	mapper, err := problem.MakeMapper(problem.InputProblemRule())
	if err != nil {
		t.Fatalf("make mapper: %v", err)
	}

	mapped := mapper.Map(nil, configErr)
	if mapped.Status != http.StatusInternalServerError || mapped.Code != "internal_error" {
		t.Fatalf("problem = %+v, want safe internal problem", mapped)
	}
}

func TestInvalidRequestDetailFollowsStatus(t *testing.T) {
	t.Parallel()

	if detail := problem.InvalidRequest(http.StatusBadRequest).Detail; detail != "Request parameters could not be decoded" {
		t.Errorf("400 detail = %q", detail)
	}

	if detail := problem.InvalidRequest(http.StatusUnprocessableEntity).Detail; detail != "Request parameters are invalid" {
		t.Errorf("422 detail = %q", detail)
	}
}

func makeJSONRequest(t *testing.T, body string, contentType string) *http.Request {
	t.Helper()

	httpRequest, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if err != nil {
		t.Fatalf("make request: %v", err)
	}

	httpRequest.Header.Set("Content-Type", contentType)

	return httpRequest
}
