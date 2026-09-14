package apitest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const defaultTimeout = 5 * time.Second

// Test executes requests through an isolated in-memory HTTP server.
// Make a separate Test for each parallel subtest.
type Test struct {
	t        testing.TB
	client   *http.Client
	timeout  time.Duration
	deadline time.Time
}

// Option configures a Test.
type Option func(*Test)

// WithTimeout sets the maximum duration of each request, including reading
// the response body. The default is five seconds.
func WithTimeout(timeout time.Duration) Option {
	return func(api *Test) {
		api.timeout = timeout
	}
}

// Make starts an in-memory server for h and registers its shutdown with
// t. The client follows no redirects and stores no cookies.
func Make(t testing.TB, h http.Handler, opts ...Option) *Test {
	t.Helper()

	if h == nil {
		t.Fatalf("HTTP handler is nil")

		return nil
	}

	api := &Test{t: t, timeout: defaultTimeout}
	for _, apply := range opts {
		if apply != nil {
			apply(api)
		}
	}

	if api.timeout <= 0 {
		t.Fatalf("request timeout must be positive, got %s", api.timeout)

		return nil
	}

	if timed, ok := t.(interface{ Deadline() (time.Time, bool) }); ok {
		api.deadline, _ = timed.Deadline()
	}

	api.client = httptest.NewTestServer(t, h).Client()
	api.client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return api
}

// requestTimeout keeps part of the remaining test time, so a hanging handler
// fails with the request in the message before the testing package aborts
// the binary.
func (api *Test) requestTimeout() time.Duration {
	if api.deadline.IsZero() {
		return api.timeout
	}

	remaining := time.Until(api.deadline)

	return min(api.timeout, remaining-min(remaining/10, time.Second))
}
