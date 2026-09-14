package request_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uchaloop/httpx/request"
)

type warehouseInput struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

func TestDecodeJSONAcceptsJSONMediaTypes(t *testing.T) {
	t.Parallel()

	for _, contentType := range []string{"application/json", "application/json; charset=utf-8", "application/problem+json"} {
		got, err := request.DecodeJSON[warehouseInput](makeJSONRequest(t, `{"name":"Minsk","id":42}`, contentType))
		if err != nil || got != (warehouseInput{Name: "Minsk", ID: 42}) {
			t.Errorf("%s: DecodeJSON = %+v, %v", contentType, got, err)
		}
	}
}

func TestDecodeJSONClassifiesInputErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		contentType string
		kind        request.DecodeErrorKind
		path        string
	}{
		{name: "empty body", contentType: "application/json", kind: request.DecodeErrorEmptyBody},
		{name: "missing media type", body: `{}`, kind: request.DecodeErrorUnsupportedMediaType},
		{name: "wrong media type", body: `{}`, contentType: "text/plain", kind: request.DecodeErrorUnsupportedMediaType},
		{name: "malformed JSON", body: `{"name":`, contentType: "application/json", kind: request.DecodeErrorMalformedJSON},
		{name: "wrong type", body: `{"id":"wrong"}`, contentType: "application/json", kind: request.DecodeErrorWrongType, path: "id"},
		{name: "unknown field", body: `{"name":"Minsk","typo":true}`, contentType: "application/json", kind: request.DecodeErrorInvalidValue},
		{name: "multiple values", body: `{} {}`, contentType: "application/json", kind: request.DecodeErrorMultipleValues},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := request.DecodeJSON[warehouseInput](
				makeJSONRequest(t, tt.body, tt.contentType),
			)

			decodeError := assertDecodeErrorKind(t, err, tt.kind)
			if decodeError.Path != tt.path {
				t.Errorf("path = %q, want %q", decodeError.Path, tt.path)
			}
		})
	}
}

func TestDecodeJSONCanAllowUnknownFieldsExplicitly(t *testing.T) {
	t.Parallel()

	got, err := request.DecodeJSON[warehouseInput](
		makeJSONRequest(t, `{"name":"Minsk","futureField":true}`, "application/json"),
		request.AllowUnknownFields(),
	)
	if err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}

	if got.Name != "Minsk" {
		t.Errorf("name = %q, want Minsk", got.Name)
	}
}

// Adapters impose the body limit; DecodeJSON must not add a hidden one.
func TestDecodeJSONDoesNotImposeBodyLimit(t *testing.T) {
	t.Parallel()

	largeName := strings.Repeat("x", 2<<20)

	got, err := request.DecodeJSON[warehouseInput](
		makeJSONRequest(t, fmt.Sprintf(`{"name":%q}`, largeName), "application/json"),
	)
	if err != nil {
		t.Fatalf("DecodeJSON large body: %v", err)
	}

	if len(got.Name) != len(largeName) {
		t.Errorf("decoded name length = %d, want %d", len(got.Name), len(largeName))
	}
}

func makeJSONRequest(t *testing.T, body, contentType string) *http.Request {
	t.Helper()

	httpRequest := httptest.NewRequest(http.MethodPost, "/warehouses", strings.NewReader(body))
	if len(contentType) > 0 {
		httpRequest.Header.Set("Content-Type", contentType)
	}

	return httpRequest
}

func assertDecodeErrorKind(t *testing.T, err error, kind request.DecodeErrorKind) *request.DecodeError {
	t.Helper()

	if err == nil {
		t.Fatalf("error is nil, want decode kind %d", kind)
	}

	decodeError, ok := errors.AsType[*request.DecodeError](err)
	if !ok {
		t.Fatalf("error type = %T, want *request.DecodeError", err)
	}

	if decodeError.Kind != kind {
		t.Errorf("decode kind = %d, want %d", decodeError.Kind, kind)
	}

	return decodeError
}
