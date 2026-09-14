package apitest_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/uchaloop/httpx/apitest"
	"github.com/uchaloop/httpx/problem"
	"github.com/uchaloop/httpx/request"
	"github.com/uchaloop/httpx/response"
)

// The examples are compiled but not run: apitest reports through the
// *testing.T of a test function, and the body of each example is the body of
// one.

type exampleArticle struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// exampleHandler is the handler under test.
func exampleHandler() http.Handler {
	mapper, err := problem.MakeMapper(problem.InputProblemRule())
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /articles/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "42" {
			_ = problem.Write(w, problem.MakeProblem(r, problem.Template{Status: http.StatusNotFound}))

			return
		}

		_ = response.Write(w, response.OK(exampleArticle{ID: 42, Title: "HTTP boundaries"}))
	})

	mux.HandleFunc("POST /articles", func(w http.ResponseWriter, r *http.Request) {
		input, err := request.DecodeJSON[exampleArticle](r)
		if err != nil {
			_ = problem.Write(w, mapper.Map(r, err))

			return
		}

		input.ID = 42
		_ = response.Write(w, response.Created("/articles/42", input))
	})

	return mux
}

func Example() {
	var t *testing.T // the t of the test function

	api := apitest.Make(t, exampleHandler())

	api.Get("/articles/42").Do().
		Status(http.StatusOK).
		Header("Content-Type", "application/json").
		JSONEqual(map[string]any{
			"data": map[string]any{"id": 42, "title": "HTTP boundaries"},
		})

	api.Get("/articles/7").Do().
		Status(http.StatusNotFound).
		Header("Content-Type", problem.ContentType).
		JSONEqual(map[string]any{
			"type":     "about:blank",
			"title":    "Not Found",
			"status":   404,
			"instance": "/articles/7",
		})
}

func ExampleRequest_JSON() {
	var t *testing.T // the t of the test function

	apitest.Make(t, exampleHandler()).Post("/articles").
		JSON(map[string]any{"title": "HTTP boundaries"}).
		Do().
		Status(http.StatusCreated).
		Header("Location", "/articles/42").
		JSONEqual(map[string]any{
			"data": map[string]any{"id": 42, "title": "HTTP boundaries"},
		})
}

func ExampleRequest_Body() {
	var t *testing.T // the t of the test function

	api := apitest.Make(t, exampleHandler())

	// Malformed JSON is sent as is.
	api.Post("/articles").Body("application/json", strings.NewReader(`{"title":`)).Do().
		Status(http.StatusBadRequest).
		Header("Content-Type", problem.ContentType)

	// So is a body in a media type the endpoint does not read.
	api.Post("/articles").Body("text/plain", strings.NewReader("title=HTTP")).Do().
		Status(http.StatusUnsupportedMediaType).
		Header("Accept", "application/json")
}

func ExampleResponse_JSON() {
	var t *testing.T // the t of the test function

	type envelope struct {
		Data exampleArticle `json:"data"`
	}

	created := apitest.Make(t, exampleHandler()).Post("/articles").
		JSON(map[string]any{"title": "HTTP boundaries"}).
		Do().
		Status(http.StatusCreated).
		JSON[envelope]()

	if created.Data.ID <= 0 {
		t.Errorf("id = %d, want a positive id", created.Data.ID)
	}
}

func ExampleWithTimeout() {
	var t *testing.T // the t of the test function

	// A slow endpoint gets more than the default five seconds.
	api := apitest.Make(t, exampleHandler(), apitest.WithTimeout(30*time.Second))

	api.Get("/articles/42").Do().Status(http.StatusOK)
}
