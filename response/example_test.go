package response_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/uchaloop/httpx/page"
	"github.com/uchaloop/httpx/response"
)

type article struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

func ExampleWrite() {
	recorder := httptest.NewRecorder()

	created := response.Created("/articles/42", article{ID: 42, Title: "HTTP boundaries"})
	if err := response.Write(recorder, created); err != nil {
		log.Fatal(err)
	}

	fmt.Println(recorder.Code)
	fmt.Println(recorder.Header().Get("Location"))
	fmt.Println(recorder.Header().Get("Content-Type"))
	fmt.Println(recorder.Body.String())
	// Output:
	// 201
	// /articles/42
	// application/json
	// {"data":{"id":42,"title":"HTTP boundaries"}}
}

func ExampleNoContent() {
	recorder := httptest.NewRecorder()
	if err := response.Write(recorder, response.NoContent()); err != nil {
		log.Fatal(err)
	}

	fmt.Println(recorder.Code, recorder.Body.Len())
	// Output: 204 0
}

func ExamplePage() {
	current, err := page.Make(page.Params{}, page.Config{DefaultSize: 2, MaxSize: 100})
	if err != nil {
		log.Fatal(err)
	}

	articles := []article{{ID: 1, Title: "API versions"}, {ID: 2, Title: "HTTP boundaries"}}

	recorder := httptest.NewRecorder()
	if err := response.Write(recorder, response.Page(articles, 5, current)); err != nil {
		log.Fatal(err)
	}

	fmt.Println(recorder.Body.String())

	// A page past the end has no items, written as an empty array.
	recorder = httptest.NewRecorder()
	if err := response.Write(recorder, response.Page[article](nil, 5, current)); err != nil {
		log.Fatal(err)
	}

	fmt.Println(recorder.Body.String())
	// Output:
	// {"data":{"items":[{"id":1,"title":"API versions"},{"id":2,"title":"HTTP boundaries"}],"total":5,"page":1,"size":2}}
	// {"data":{"items":[],"total":5,"page":1,"size":2}}
}

func ExampleResponse_SetHeaders() {
	answer := response.OK(article{ID: 42, Title: "HTTP boundaries"})
	answer.Header = http.Header{"x-user": {"7"}}

	// A router writes the headers into its own response the same way.
	header := make(http.Header)
	answer.SetHeaders(header)

	fmt.Println(header)
	// Output: map[Content-Type:[application/json] X-User:[7]]
}

func ExampleResponse_Validate() {
	answer := response.Response{
		Status:      http.StatusCreated,
		ContentType: response.ContentTypeJSON,
		Body:        article{ID: 42, Title: "HTTP boundaries"},
	}

	fmt.Println(answer.Validate())
	// Output: status 201 requires a location
}
