package problem_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/uchaloop/httpx/page"
	"github.com/uchaloop/httpx/problem"
	"github.com/uchaloop/httpx/request"
)

var errNotFound = errors.New("article not found")

type createArticle struct {
	Title string `json:"title"`
}

type quotaError struct {
	limit int
}

func (e *quotaError) Error() string {
	return fmt.Sprintf("quota of %d requests exceeded", e.limit)
}

func ExampleMakeMapper() {
	mapper, err := problem.MakeMapper(
		problem.WhenIs(errNotFound, problem.Template{
			Status: http.StatusNotFound,
			Code:   "article_not_found",
			Detail: "Article was not found",
		}),
		problem.InputProblemRule(),
		problem.ErrorRule(),
	)
	if err != nil {
		log.Fatal(err)
	}

	getArticle := func(w http.ResponseWriter, r *http.Request) {
		// The service wraps its error; errors.Is still finds it.
		err := fmt.Errorf("get article 42: %w", errNotFound)
		if writeErr := problem.Write(w, mapper.Map(r, err)); writeErr != nil {
			log.Print(writeErr)
		}
	}

	recorder := httptest.NewRecorder()
	getArticle(recorder, httptest.NewRequest(http.MethodGet, "/articles/42", nil))

	fmt.Println(recorder.Code, recorder.Header().Get("Content-Type"))
	fmt.Println(recorder.Body.String())
	// Output:
	// 404 application/problem+json
	// {"type":"about:blank","title":"Not Found","status":404,"detail":"Article was not found","instance":"/articles/42","code":"article_not_found"}
}

func ExampleMapper_Map() {
	mapper, err := problem.MakeMapper()
	if err != nil {
		log.Fatal(err)
	}

	// No rule knows this error, so the client learns nothing about it.
	r := httptest.NewRequest(http.MethodGet, "/articles/42", nil)
	mapped := mapper.Map(r, errors.New("dial tcp 10.0.0.5:5432: connection refused"))

	body, err := json.Marshal(mapped)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(body))
	// Output:
	// {"type":"about:blank","title":"Internal Server Error","status":500,"instance":"/articles/42","code":"internal_error"}
}

func ExampleWhenAs() {
	mapper, err := problem.MakeMapper(problem.WhenAs(func(quota *quotaError) problem.Template {
		return problem.Template{
			Status: http.StatusTooManyRequests,
			Code:   "quota_exceeded",
			Detail: fmt.Sprintf("At most %d requests per minute are allowed", quota.limit),
			Header: http.Header{"Retry-After": {"60"}},
		}
	}))
	if err != nil {
		log.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/articles", nil)
	mapped := mapper.Map(r, fmt.Errorf("create article: %w", &quotaError{limit: 100}))

	fmt.Println(mapped.Status, mapped.Code)
	fmt.Println(mapped.Detail)
	fmt.Println("Retry-After:", mapped.Header.Get("Retry-After"))
	// Output:
	// 429 quota_exceeded
	// At most 100 requests per minute are allowed
	// Retry-After: 60
}

func ExampleInputProblemRule() {
	mapper, err := problem.MakeMapper(problem.InputProblemRule())
	if err != nil {
		log.Fatal(err)
	}

	show := func(mapped problem.Problem) {
		body, err := json.Marshal(mapped)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(body))
	}

	// A JSON value of the wrong type cannot be decoded: 400.
	r := httptest.NewRequest(http.MethodPost, "/articles", strings.NewReader(`{"title":5}`))
	r.Header.Set("Content-Type", "application/json")

	_, err = request.DecodeJSON[createArticle](r)
	show(mapper.Map(r, err))

	// A page size over the maximum breaks a constraint: 422.
	size := uint64(500)
	r = httptest.NewRequest(http.MethodGet, "/articles?size=500", nil)

	_, err = page.Make(page.Params{Size: &size}, page.Config{DefaultSize: 20, MaxSize: 100})
	show(mapper.Map(r, err))
	// Output:
	// {"type":"about:blank","title":"Bad Request","status":400,"detail":"Request body must contain one valid JSON document","instance":"/articles","code":"invalid_json","errors":[{"path":"title","code":"type","message":"title must be a string"}]}
	// {"type":"about:blank","title":"Unprocessable Entity","status":422,"detail":"Request parameters are invalid","instance":"/articles","code":"invalid_request","errors":[{"path":"size","code":"max","message":"size must be 100 or less"}]}
}

func ExampleMakeInvalidRequest() {
	mapper, err := problem.MakeMapper(problem.ErrorRule())
	if err != nil {
		log.Fatal(err)
	}

	// A handler that checks a value itself reports it as a validator would.
	invalid := problem.MakeInvalidRequest(http.StatusUnprocessableEntity, nil, problem.InvalidParam{
		Path:    "title",
		Code:    "required",
		Message: "title is a required field",
	})

	r := httptest.NewRequest(http.MethodPost, "/articles", nil)

	body, err := json.Marshal(mapper.Map(r, invalid))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(body))
	// Output:
	// {"type":"about:blank","title":"Unprocessable Entity","status":422,"detail":"Request parameters are invalid","instance":"/articles","code":"invalid_request","errors":[{"path":"title","code":"required","message":"title is a required field"}]}
}

func ExampleMakeProblem() {
	// A router's own 405 has a status and a header, but no code or detail.
	r := httptest.NewRequest(http.MethodDelete, "/articles", nil)
	mapped := problem.MakeProblem(r, problem.Template{
		Status: http.StatusMethodNotAllowed,
		Header: http.Header{"Allow": {"GET, POST"}},
	})

	recorder := httptest.NewRecorder()
	if err := problem.Write(recorder, mapped); err != nil {
		log.Fatal(err)
	}

	fmt.Println(recorder.Code, recorder.Header().Get("Allow"))
	fmt.Println(recorder.Body.String())
	// Output:
	// 405 GET, POST
	// {"type":"about:blank","title":"Method Not Allowed","status":405,"instance":"/articles"}
}
