package request_test

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/uchaloop/httpx/request"
)

type createArticle struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// titleValidator stands in for a validation library behind the Validator
// interface.
type titleValidator struct{}

func (titleValidator) Validate(value any) error {
	if input, ok := value.(*createArticle); ok && len(strings.TrimSpace(input.Title)) == 0 {
		return errors.New("title is a required field")
	}

	return nil
}

func jsonRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/articles", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	return r
}

func ExampleDecodeJSON() {
	r := httptest.NewRequest(
		http.MethodPost,
		"/articles",
		strings.NewReader(`{"title":"HTTP boundaries","status":"draft"}`),
	)

	r.Header.Set("Content-Type", "application/json")

	input, err := request.DecodeJSON[createArticle](r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", input)
	// Output: {Title:HTTP boundaries Status:draft}
}

func ExampleDecodeJSON_bodyLimit() {
	r := jsonRequest(`{"title":"` + strings.Repeat("a", 64) + `"}`)

	// DecodeJSON sets no limit, and the error of this one reaches the caller
	// as is.
	r.Body = http.MaxBytesReader(nil, r.Body, 32)

	_, err := request.DecodeJSON[createArticle](r)
	if tooLarge, ok := errors.AsType[*http.MaxBytesError](err); ok {
		fmt.Println("body is over", tooLarge.Limit, "bytes")
	}
	// Output: body is over 32 bytes
}

func ExampleDecodeError() {
	_, err := request.DecodeJSON[createArticle](jsonRequest(`{"title":5}`))
	if decodeErr, ok := errors.AsType[*request.DecodeError](err); ok && decodeErr.Kind == request.DecodeErrorWrongType {
		fmt.Println("wrong type at", decodeErr.Path)
	}
	// Output: wrong type at title
}

func ExampleAllowUnknownFields() {
	const body = `{"title":"HTTP boundaries","author":"Ada"}`

	_, err := request.DecodeJSON[createArticle](jsonRequest(body))
	fmt.Println(err)

	input, err := request.DecodeJSON[createArticle](jsonRequest(body), request.AllowUnknownFields())
	fmt.Println(input.Title, err)
	// Output:
	// invalid JSON value: json: unknown field "author"
	// HTTP boundaries <nil>
}

func ExampleDecodeAndValidateJSON() {
	r := jsonRequest(`{"title":" ","status":"draft"}`)

	_, err := request.DecodeAndValidateJSON[createArticle](r, titleValidator{})
	fmt.Println(err)
	// Output: validate decoded JSON: title is a required field
}
