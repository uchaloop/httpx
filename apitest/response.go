package apitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"unicode/utf8"
)

// diagnosticLimit bounds a printed body, not the comparison.
const diagnosticLimit = 8 << 10

// Response holds a buffered response. Assertions and Raw never consume it.
type Response struct {
	t        testing.TB
	label    string
	response *http.Response
	body     []byte

	decoded   bool
	document  any
	decodeErr error
}

// Raw returns a copy of the response with its own reader over the buffered
// body.
func (r *Response) Raw() *http.Response {
	raw := *r.response
	raw.Header = r.response.Header.Clone()
	raw.Trailer = r.response.Trailer.Clone()
	raw.Body = io.NopCloser(bytes.NewReader(r.body))

	return &raw
}

// Status asserts the status code. A mismatch stops the test and prints the
// body: a response with another status has another shape, so later assertions
// would only add noise.
func (r *Response) Status(code int) *Response {
	r.t.Helper()

	if r.response.StatusCode != code {
		r.t.Fatalf(
			"%s\nstatus: expected %s, got %s\n%s",
			r.label,
			statusText(code),
			statusText(r.response.StatusCode),
			r.describeBody(),
		)
	}

	return r
}

// Header asserts all values of a response header, in order.
func (r *Response) Header(name string, values ...string) *Response {
	r.t.Helper()

	if len(values) == 0 {
		r.t.Fatalf("%s: header %q requires an expected value; use HeaderAbsent", r.label, name)

		return r
	}

	if actual := r.response.Header.Values(name); !slices.Equal(actual, values) {
		r.errorf("header %q: expected %s, got %s", name, formatValues(values), formatValues(actual))
	}

	return r
}

// HeaderAbsent asserts that the response has no such header.
func (r *Response) HeaderAbsent(name string) *Response {
	r.t.Helper()

	if actual := r.response.Header.Values(name); len(actual) > 0 {
		r.errorf("header %q: expected absence, got %s", name, formatValues(actual))
	}

	return r
}

// BodyEqual asserts the exact body. BodyEqual("") asserts an empty body.
func (r *Response) BodyEqual(expected string) *Response {
	r.t.Helper()

	if actual := string(r.body); actual != expected {
		r.errorf("body differs\nexpected:\n%s\nactual:\n%s", formatText(expected), formatText(actual))
	}

	return r
}

// JSON decodes the body into T as a client using encoding/json would: unknown
// fields are allowed and the body must hold exactly one JSON value. A decode
// failure stops the test.
func (r *Response) JSON[T any]() T {
	r.t.Helper()

	var value T
	if err := json.Unmarshal(r.body, &value); err != nil {
		r.t.Fatalf("%s\ncannot decode JSON: %v\n%s", r.label, err, r.describeBody())
	}

	return value
}

// JSONEqual asserts that the body is the same JSON document as expected
// encoded by encoding/json. Object key order and whitespace are ignored.
// Array order, extra and missing fields, null and number literals are not: 1
// and 1.0 differ, as they do for a typed client. A body that is not exactly
// one JSON document, or has duplicate object keys, stops the test.
func (r *Response) JSONEqual(expected any) *Response {
	r.t.Helper()

	encoded, err := json.Marshal(expected)
	if err != nil {
		r.t.Fatalf("%s: cannot encode expected JSON: %v", r.label, err)

		return r
	}

	want, err := decodeJSON(encoded)
	if err != nil {
		r.t.Fatalf("%s: expected JSON is ambiguous: %v", r.label, err)

		return r
	}

	got, err := r.decodedJSON()
	if err != nil {
		r.t.Fatalf("%s\nresponse is not one unambiguous JSON document: %v\n%s", r.label, err, r.describeBody())

		return r
	}

	if differences := diffJSON(want, got); len(differences) > 0 {
		r.errorf("JSON differs:\n%s\n%s", joinLines(differences), r.describeBody())
	}

	return r
}

func (r *Response) decodedJSON() (any, error) {
	if !r.decoded {
		r.decoded = true
		r.document, r.decodeErr = decodeJSON(r.body)
	}

	return r.document, r.decodeErr
}

func (r *Response) errorf(format string, args ...any) {
	r.t.Helper()
	r.t.Errorf("%s\n%s", r.label, fmt.Sprintf(format, args...))
}

func (r *Response) describeBody() string {
	title := "body"
	if contentType := r.response.Header.Get("Content-Type"); len(contentType) > 0 {
		title += " (" + contentType + ")"
	}

	return title + ":\n" + formatBody(r.body)
}

// formatBody indents a JSON body; any other body is printed as is.
func formatBody(body []byte) string {
	var indented bytes.Buffer
	if json.Indent(&indented, body, "", "  ") == nil {
		body = indented.Bytes()
	}

	return formatText(string(body))
}

func formatText(text string) string {
	if len(text) == 0 {
		return "<empty>"
	}

	return clip(text, diagnosticLimit)
}

func formatValues(values []string) string {
	switch len(values) {
	case 0:
		return "<absent>"
	case 1:
		return strconv.Quote(values[0])
	default:
		return fmt.Sprintf("%q", values)
	}
}

func statusText(code int) string {
	if text := http.StatusText(code); len(text) > 0 {
		return strconv.Itoa(code) + " " + text
	}

	return strconv.Itoa(code)
}

// clip cuts text at a rune boundary and reports how much was left out.
func clip(text string, limit int) string {
	if len(text) <= limit {
		return text
	}

	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}

	return fmt.Sprintf("%s… [%d more bytes]", text[:cut], len(text)-cut)
}
