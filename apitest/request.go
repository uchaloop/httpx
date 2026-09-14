package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const (
	contentTypeJSON = "application/json"
	defaultHost     = "example.com"
)

// Request is a single-use request builder. Nothing is sent before Do. It is
// not safe for concurrent use.
type Request struct {
	api         *Test
	method      string
	target      string
	host        string
	header      http.Header
	query       url.Values
	cookies     []*http.Cookie
	body        io.Reader
	contentType string
	ctx         context.Context
	sent        bool
}

// Request starts a request with any method. The path must be local and
// absolute. A query string in the path is sent unchanged, so it may be
// malformed on purpose.
func (api *Test) Request(method, path string) *Request {
	api.t.Helper()

	return &Request{
		api:    api,
		method: method,
		target: path,
		header: make(http.Header),
		query:  make(url.Values),
		ctx:    context.Background(),
	}
}

// Get starts a GET request.
func (api *Test) Get(path string) *Request {
	api.t.Helper()

	return api.Request(http.MethodGet, path)
}

// Post starts a POST request.
func (api *Test) Post(path string) *Request {
	api.t.Helper()

	return api.Request(http.MethodPost, path)
}

// Put starts a PUT request.
func (api *Test) Put(path string) *Request {
	api.t.Helper()

	return api.Request(http.MethodPut, path)
}

// Patch starts a PATCH request.
func (api *Test) Patch(path string) *Request {
	api.t.Helper()

	return api.Request(http.MethodPatch, path)
}

// Delete starts a DELETE request.
func (api *Test) Delete(path string) *Request {
	api.t.Helper()

	return api.Request(http.MethodDelete, path)
}

// Head starts a HEAD request. A client never receives a HEAD response body.
func (api *Test) Head(path string) *Request {
	api.t.Helper()

	return api.Request(http.MethodHead, path)
}

// Options starts an OPTIONS request.
func (api *Test) Options(path string) *Request {
	api.t.Helper()

	return api.Request(http.MethodOptions, path)
}

// Header sets all values of a request header, replacing earlier ones. An
// explicit Content-Type overrides the media type set by JSON or Body. Use Host
// for the Host header: net/http ignores it in Header.
func (r *Request) Header(name string, values ...string) *Request {
	r.api.t.Helper()

	key := http.CanonicalHeaderKey(name)
	if key == "Host" {
		r.api.t.Fatalf("%s: set the Host header with Host", r.label())

		return r
	}

	if len(values) == 0 {
		r.api.t.Fatalf("%s: header %q requires a value", r.label(), name)

		return r
	}

	r.header[key] = slices.Clone(values)

	return r
}

// Host sets the Host header, for example for host-based routing. The default
// is example.com.
func (r *Request) Host(host string) *Request {
	r.api.t.Helper()

	if len(host) == 0 {
		r.api.t.Fatalf("%s: host is empty", r.label())

		return r
	}

	r.host = host

	return r
}

// Query appends values of a query parameter after the query written in the
// path. Parameters added here are encoded in name order.
func (r *Request) Query(name string, values ...string) *Request {
	r.api.t.Helper()

	if len(values) == 0 {
		r.api.t.Fatalf("%s: query parameter %q requires a value", r.label(), name)

		return r
	}

	r.query[name] = append(r.query[name], values...)

	return r
}

// Cookie adds a copy of cookie to the request.
func (r *Request) Cookie(cookie *http.Cookie) *Request {
	r.api.t.Helper()

	if cookie == nil {
		r.api.t.Fatalf("%s: cookie is nil", r.label())

		return r
	}

	copied := *cookie
	r.cookies = append(r.cookies, &copied)

	return r
}

// Context sets the parent context of the request, for example one that a mock
// cancels on an unexpected call. Cancellation reaches the handler; context
// values do not cross the HTTP connection.
func (r *Request) Context(ctx context.Context) *Request {
	r.api.t.Helper()

	if ctx == nil {
		r.api.t.Fatalf("%s: context is nil", r.label())

		return r
	}

	r.ctx = ctx

	return r
}

// JSON sets the body to value encoded by encoding/json, with media type
// application/json. A request has at most one body.
func (r *Request) JSON(value any) *Request {
	r.api.t.Helper()

	body, err := json.Marshal(value)
	if err != nil {
		r.api.t.Fatalf("%s: cannot encode request JSON: %v", r.label(), err)

		return r
	}

	return r.setBody(contentTypeJSON, bytes.NewReader(body))
}

// Body sets a raw body, for example malformed JSON, with media type
// contentType. An empty contentType sends no Content-Type. A request has at
// most one body.
func (r *Request) Body(contentType string, body io.Reader) *Request {
	r.api.t.Helper()

	if body == nil {
		r.api.t.Fatalf("%s: body reader is nil", r.label())

		return r
	}

	return r.setBody(contentType, body)
}

func (r *Request) setBody(contentType string, body io.Reader) *Request {
	r.api.t.Helper()

	if r.body != nil {
		r.api.t.Fatalf("%s: request body is already set", r.label())

		return r
	}

	r.body = body
	r.contentType = contentType

	return r
}

// Do sends the request once, then reads and closes the complete response
// body. Transport, timeout and read failures stop the test.
func (r *Request) Do() *Response {
	r.api.t.Helper()

	if r.sent {
		r.api.t.Fatalf("%s: request was already sent", r.label())

		return nil
	}

	r.sent = true

	target, err := r.url()
	if err != nil {
		r.api.t.Fatalf("%s: %v", r.label(), err)

		return nil
	}

	timeout := r.api.requestTimeout()
	ctx, cancel := context.WithTimeout(r.ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, r.method, target, r.body)
	if err != nil {
		r.api.t.Fatalf("%s: cannot create request: %v", r.label(), err)

		return nil
	}

	request.Header = r.header.Clone()
	if len(r.contentType) > 0 && len(request.Header.Values("Content-Type")) == 0 {
		request.Header.Set("Content-Type", r.contentType)
	}

	if len(r.host) > 0 {
		request.Host = r.host
	}

	for _, cookie := range r.cookies {
		request.AddCookie(cookie)
	}

	label := r.method + " " + request.URL.RequestURI()
	response, err := r.api.client.Do(request)
	if err != nil {
		r.api.t.Fatalf("%s: %s", label, transportFailure(ctx, timeout, err))

		return nil
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		r.api.t.Fatalf("%s: cannot read response body: %s", label, transportFailure(ctx, timeout, err))

		return nil
	}

	return &Response{t: r.api.t, label: label, response: response, body: body}
}

// url keeps the query written in the path verbatim and appends the query
// parameters added with Query.
func (r *Request) url() (string, error) {
	target, err := url.Parse(r.target)
	if err != nil {
		return "", err
	}

	if len(target.Scheme) > 0 || len(target.Host) > 0 || target.User != nil ||
		len(target.Fragment) > 0 || !strings.HasPrefix(target.Path, "/") {
		return "", fmt.Errorf("path %q must be local and absolute, without scheme, host or fragment", r.target)
	}

	target.Scheme = "http"
	target.Host = defaultHost
	if len(r.query) > 0 {
		if len(target.RawQuery) > 0 {
			target.RawQuery += "&"
		}

		target.RawQuery += r.query.Encode()
	}

	return target.String(), nil
}

func (r *Request) label() string {
	return r.method + " " + r.target
}

func transportFailure(ctx context.Context, timeout time.Duration, err error) string {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Sprintf("request deadline exceeded (timeout %s): %v", timeout.Round(time.Millisecond), err)
	case errors.Is(ctx.Err(), context.Canceled):
		return fmt.Sprintf("request canceled: %v", err)
	default:
		return err.Error()
	}
}
