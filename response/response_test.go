package response_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/uchaloop/httpx/apitest"
	"github.com/uchaloop/httpx/page"
	"github.com/uchaloop/httpx/response"
)

func writeHandler(t *testing.T, answer response.Response) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := response.Write(w, answer); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
}

func TestWriteSendsConstructedResponses(t *testing.T) {
	t.Parallel()

	currentPage, err := page.Make(page.Params{}, page.Config{DefaultSize: 20, MaxSize: 100})
	if err != nil {
		t.Fatalf("make page: %v", err)
	}

	item := struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{ID: 42, Name: "Minsk"}

	apitest.Make(t, writeHandler(t, response.JSON(http.StatusAccepted, item))).Get("/").Do().
		Status(http.StatusAccepted).
		Header("Content-Type", "application/json").
		JSONEqual(map[string]any{"id": 42, "name": "Minsk"})

	apitest.Make(t, writeHandler(t, response.Data(http.StatusOK, nil))).Get("/").Do().
		Status(http.StatusOK).
		JSONEqual(map[string]any{"data": nil})

	apitest.Make(t, writeHandler(t, response.OK(item))).Get("/").Do().
		Status(http.StatusOK).
		JSONEqual(map[string]any{"data": map[string]any{"id": 42, "name": "Minsk"}})

	apitest.Make(t, writeHandler(t, response.Created("/warehouses/42", item))).Get("/").Do().
		Status(http.StatusCreated).
		Header("Location", "/warehouses/42").
		JSONEqual(map[string]any{"data": map[string]any{"id": 42, "name": "Minsk"}})

	apitest.Make(t, writeHandler(t, response.Page[string](nil, 17, currentPage))).Get("/").Do().
		Status(http.StatusOK).
		JSONEqual(map[string]any{"data": map[string]any{"items": []any{}, "total": 17, "page": 1, "size": 20}})

	apitest.Make(t, writeHandler(t, response.NoContent())).Get("/").Do().
		Status(http.StatusNoContent).
		HeaderAbsent("Content-Type").
		BodyEqual("")
}

func TestInvalidResponsesAreNotWritten(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		answer response.Response
	}{
		{name: "status 0", answer: response.JSON(0, 1)},
		{name: "status 100", answer: response.JSON(http.StatusContinue, 1)},
		{name: "status 600", answer: response.JSON(600, 1)},
		{name: "body with 204", answer: response.JSON(http.StatusNoContent, 1)},
		{name: "body with 304", answer: response.JSON(http.StatusNotModified, 1)},
		{name: "body without content type", answer: response.Response{Status: http.StatusOK, Body: 1}},
		{name: "empty location", answer: response.Created("", 1)},
		{name: "encoding failure", answer: response.Created("/articles/1", make(chan int))},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			if err := response.Write(recorder, test.answer); err == nil {
				t.Fatal("Write succeeded, want error")
			}

			if recorder.Code != http.StatusOK || recorder.Body.Len() > 0 || len(recorder.Header()) > 0 {
				t.Errorf("writer changed: status %d, header %v, body %q", recorder.Code, recorder.Header(), recorder.Body.String())
			}
		})
	}
}

type errWriter struct {
	header http.Header
	status int
	err    error
}

func (w *errWriter) Header() http.Header {
	return w.header
}

func (w *errWriter) WriteHeader(status int) {
	w.status = status
}

func (w *errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestWriteReturnsWriterError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("connection closed")
	writer := &errWriter{header: make(http.Header), err: writeErr}

	err := response.Write(writer, response.OK(true))
	if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "write JSON") {
		t.Fatalf("error = %v, want wrapped write error", err)
	}

	if writer.status != http.StatusOK {
		t.Errorf("status = %d, want 200", writer.status)
	}
}

// A custom header keeps one canonical name however it is spelled, so a
// middleware that reads or sets X-User sees the same header, never a second one.
func TestWriteCanonicalizesCustomHeaders(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-User", "middleware")
	answer := response.NoContent()
	answer.Header = http.Header{"x-user": {"42"}}

	if err := response.Write(recorder, answer); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := recorder.Header().Values("X-User"); !slices.Equal(got, []string{"42"}) {
		t.Fatalf("X-User = %q, want the response's single value", got)
	}
}
