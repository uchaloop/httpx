package request_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/uchaloop/httpx/request"
)

func makeBodyRequest(body io.Reader) *http.Request {
	httpRequest := httptest.NewRequest(http.MethodPost, "/", body)
	httpRequest.Header.Set("Content-Type", "application/json")

	return httpRequest
}

func TestDecodeJSONReturnsBodyReadFailureAsIs(t *testing.T) {
	t.Parallel()

	httpRequest := makeBodyRequest(strings.NewReader(`{"title":"too long"}`))
	httpRequest.Body = http.MaxBytesReader(httptest.NewRecorder(), httpRequest.Body, 4)

	_, err := request.DecodeJSON[map[string]any](httpRequest)
	_, isTooLarge := errors.AsType[*http.MaxBytesError](err)
	_, isDecodeErr := errors.AsType[*request.DecodeError](err)
	if !isTooLarge || isDecodeErr {
		t.Fatalf("error = %v, want the reader's *http.MaxBytesError, not a DecodeError", err)
	}
}

func TestDecodeJSONReportsTruncatedBodyAsMalformed(t *testing.T) {
	t.Parallel()

	body := io.MultiReader(strings.NewReader(`{"title":`), iotest.ErrReader(io.ErrUnexpectedEOF))
	_, err := request.DecodeJSON[map[string]any](makeBodyRequest(body))
	if decodeErr, ok := errors.AsType[*request.DecodeError](err); !ok || decodeErr.Kind != request.DecodeErrorMalformedJSON {
		t.Fatalf("error = %v, want a malformed JSON DecodeError", err)
	}
}
