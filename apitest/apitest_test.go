package apitest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uchaloop/httpx/apitest"
)

// fakeT records failures instead of failing the real test. Fatalf panics with
// fatal so expectFatal can observe it; cleanup goes to the real test.
type fakeT struct {
	testing.TB

	mu     sync.Mutex
	errors []string
}

type fatal string

func makeFake(t *testing.T) *fakeT {
	return &fakeT{TB: t}
}

func (f *fakeT) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeT) Fatalf(format string, args ...any) {
	panic(fatal(fmt.Sprintf(format, args...)))
}

func (f *fakeT) failures() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.errors)
}

// expectFatal runs step and returns the message it stopped the test with.
func expectFatal(t *testing.T, step func()) string {
	t.Helper()

	message, stopped := func() (message string, stopped bool) {
		defer func() {
			if value := recover(); value != nil {
				stop, ok := value.(fatal)
				if !ok {
					panic(value)
				}

				message, stopped = string(stop), true
			}
		}()

		step()

		return "", false
	}()
	if !stopped {
		t.Fatal("step did not stop the test")
	}

	return message
}

func expectContains(t *testing.T, message string, parts ...string) {
	t.Helper()

	for _, part := range parts {
		if !strings.Contains(message, part) {
			t.Errorf("message does not contain %q:\n%s", part, message)
		}
	}
}

func jsonHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

func TestRequestSendsInputOnlyOnDo(t *testing.T) {
	t.Parallel()

	type captured struct {
		Method, URI, Host, Body, ContentType, Cookie string
		Header                                       string
	}

	received := make(chan captured, 1)
	api := apitest.Make(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		cookie, err := r.Cookie("session")
		if err != nil {
			t.Errorf("read cookie: %v", err)

			return
		}

		received <- captured{
			Method:      r.Method,
			URI:         r.RequestURI,
			Host:        r.Host,
			Body:        string(body),
			ContentType: r.Header.Get("Content-Type"),
			Cookie:      cookie.Value,
			Header:      strings.Join(r.Header.Values("X-Test"), ","),
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	request := api.Post("/items?flag&b=2&a=%zz").
		Query("sort", "id", "title").
		Header("X-Test", "old").
		Header("X-Test", "first", "second").
		Host("tenant.example.com").
		Cookie(&http.Cookie{Name: "session", Value: "cookie"}).
		JSON(map[string]string{"name": "Ada"})

	select {
	case <-received:
		t.Fatal("request was sent before Do")
	default:
	}

	request.Do().Status(http.StatusNoContent)

	want := captured{
		Method:      http.MethodPost,
		URI:         "/items?flag&b=2&a=%zz&sort=id&sort=title",
		Host:        "tenant.example.com",
		Body:        `{"name":"Ada"}`,
		ContentType: "application/json",
		Cookie:      "cookie",
		Header:      "first,second",
	}
	if got := <-received; got != want {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
}

func TestRequestIsSingleUse(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	api := apitest.Make(makeFake(t), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))

	request := api.Get("/items")
	request.Do()

	expectContains(t, expectFatal(t, func() { request.Do() }), "GET /items", "already sent")
	if calls.Load() != 1 {
		t.Fatalf("handler calls = %d, want 1", calls.Load())
	}
}

func TestBodyMediaType(t *testing.T) {
	t.Parallel()

	api := apitest.Make(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if values := r.Header.Values("Content-Type"); len(values) > 0 {
			w.Header()["X-Received-Type"] = values
		}

		_, _ = io.Copy(w, r.Body)
	}))

	api.Post("/").JSON(1).Do().
		Header("X-Received-Type", "application/json").BodyEqual("1")

	api.Patch("/").Header("Content-Type", "application/merge-patch+json").JSON(1).Do().
		Header("X-Received-Type", "application/merge-patch+json")

	api.Patch("/").JSON(1).Header("Content-Type", "application/merge-patch+json").Do().
		Header("X-Received-Type", "application/merge-patch+json")

	api.Post("/").Body("", strings.NewReader("{")).Do().
		HeaderAbsent("X-Received-Type").BodyEqual("{")

	api.Post("/").Body("text/plain", strings.NewReader("raw")).Do().
		Header("X-Received-Type", "text/plain").BodyEqual("raw")

	fake := apitest.Make(makeFake(t), http.NotFoundHandler())
	expectContains(t, expectFatal(t, func() { fake.Post("/").JSON(1).Body("", strings.NewReader("")) }), "already set")
	expectContains(t, expectFatal(t, func() { fake.Post("/").Body("", nil) }), "reader is nil")
}

func TestInvalidRequestInputStops(t *testing.T) {
	t.Parallel()

	api := apitest.Make(makeFake(t), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("invalid request reached the handler")
	}))

	expectContains(t, expectFatal(t, func() { api.Get("/").Header("host", "tenant.example.com") }), "with Host")
	expectContains(t, expectFatal(t, func() { api.Get("/").Header("X-Test") }), `header "X-Test" requires a value`)
	expectContains(t, expectFatal(t, func() { api.Get("/").Query("sort") }), `"sort" requires a value`)
	expectContains(t, expectFatal(t, func() { api.Get("/").Host("") }), "host is empty")
	expectContains(t, expectFatal(t, func() { api.Get("/").Cookie(nil) }), "cookie is nil")
	for _, target := range []string{"https://external.test/", "//external.test/items", "/items#part", "items", ""} {
		expectContains(t, expectFatal(t, func() { api.Get(target).Do() }), "must be local and absolute")
	}
}

func TestRedirectsAndCookiesAreNotFollowed(t *testing.T) {
	t.Parallel()

	var redirected atomic.Bool
	api := apitest.Make(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "value"})
			http.Redirect(w, r, "/destination", http.StatusFound)
		case "/destination":
			redirected.Store(true)
		case "/cookies":
			if len(r.Cookies()) > 0 {
				t.Error("client stored cookies")
			}

			w.WriteHeader(http.StatusNoContent)
		}
	}))

	api.Get("/redirect").Do().Status(http.StatusFound).Header("Location", "/destination")
	if redirected.Load() {
		t.Fatal("redirect was followed")
	}

	api.Get("/cookies").Do().Status(http.StatusNoContent).BodyEqual("")
}

func TestRawIsIndependent(t *testing.T) {
	t.Parallel()

	response := apitest.Make(t, jsonHandler(http.StatusOK, `{"id":42}`)).Get("/items").Do()
	first := response.Raw()
	first.Header.Set("Content-Type", "changed")
	_, _ = io.ReadAll(first.Body)
	_ = first.Body.Close()

	body, err := io.ReadAll(response.Raw().Body)
	if err != nil || string(body) != `{"id":42}` {
		t.Fatalf("second Raw body = %q, %v", body, err)
	}

	response.Header("Content-Type", "application/json").BodyEqual(`{"id":42}`).JSONEqual(map[string]int{"id": 42})
}

func TestTimeoutAndCancellation(t *testing.T) {
	t.Parallel()

	type key struct{}
	canceled := make(chan struct{}, 1)
	api := apitest.Make(makeFake(t), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Context().Value(key{}) != nil {
			t.Error("client context value crossed HTTP")
		}

		<-r.Context().Done()
		canceled <- struct{}{}
	}), apitest.WithTimeout(30*time.Millisecond))

	ctx := context.WithValue(context.Background(), key{}, "value")
	expectContains(t, expectFatal(t, func() { api.Get("/slow").Context(ctx).Do() }),
		"GET /slow", "deadline exceeded (timeout 30ms)")

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("handler did not observe cancellation")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	expectContains(t, expectFatal(t, func() { api.Get("/slow").Context(ctx).Do() }), "request canceled")
}

func TestTransportFailureIsReported(t *testing.T) {
	t.Parallel()

	api := apitest.Make(makeFake(t), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "short")
	}))

	expectContains(t, expectFatal(t, func() { api.Get("/").Do() }), "cannot read response body", "unexpected EOF")
}

func TestStatusMismatchStopsWithBody(t *testing.T) {
	t.Parallel()

	fake := makeFake(t)
	response := apitest.Make(fake, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"status":422,"code":"invalid_request"}`)
	})).Post("/articles?page=0").JSON(map[string]string{"title": ""}).Do()

	message := expectFatal(t, func() { response.Status(http.StatusCreated).JSONEqual(map[string]int{"id": 42}) })
	expectContains(t, message,
		"POST /articles?page=0",
		"status: expected 201 Created, got 422 Unprocessable Entity",
		"body (application/problem+json):",
		`"code": "invalid_request"`,
	)

	if failures := fake.failures(); len(failures) > 0 {
		t.Fatalf("assertions after Status reported: %q", failures)
	}
}

func TestJSONEqualReportsEveryDifference(t *testing.T) {
	t.Parallel()

	fake := makeFake(t)
	apitest.Make(fake, jsonHandler(http.StatusOK, `{
		"id": 42, "name": "Jack", "createdAt": "2026-01-01",
		"tags": ["a", "b", "c"], "price": "10", "build.version": 1
	}`)).Get("/articles/42").Do().
		JSONEqual(map[string]any{
			"id": 42, "name": "John", "owner": nil,
			"tags": []string{"a", "b"}, "price": 10, "build.version": 2,
		})

	failures := fake.failures()
	if len(failures) != 1 {
		t.Fatalf("failures = %q, want one", failures)
	}

	expectContains(t, failures[0],
		"GET /articles/42\nJSON differs:",
		`$["build.version"]: expected 2, got 1`,
		`$.createdAt: unexpected field, got "2026-01-01"`,
		`$.name: expected "John", got "Jack"`,
		`$.owner: missing field, expected null`,
		`$.price: expected number 10, got string "10"`,
		`$.tags[2]: unexpected element, got "c"`,
		"body (application/json):",
	)
}

func TestJSONEqualLimitsReportedDifferences(t *testing.T) {
	t.Parallel()

	fake := makeFake(t)
	apitest.Make(fake, jsonHandler(http.StatusOK, `[]`)).Get("/").Do().
		JSONEqual(make([]int, 15))

	failures := fake.failures()
	if len(failures) != 1 || strings.Count(failures[0], "missing element") != 10 {
		t.Fatalf("failures = %q, want ten reported differences", failures)
	}

	expectContains(t, failures[0], "… and 5 more")
}

func TestJSONEqualSemantics(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		body     string
		expected any
		equal    bool
	}{
		{`{"a":1,"b":[true,null]}`, map[string]any{"b": []any{true, nil}, "a": 1}, true},
		{" {\n  \"a\" : 1 }\n", map[string]int{"a": 1}, true},
		{`"<a&b>"`, "<a&b>", true},
		{`9007199254740993`, int64(9007199254740993), true},
		{`9007199254740993`, int64(9007199254740992), false},
		{`42`, json.Number("42"), true},
		{`1.0`, 1, false},
		{`1e0`, 1, false},
		{`null`, map[string]any{}, false},
		{`{"x":null}`, map[string]any{}, false},
		{`[]`, nil, false},
		{`[1,2]`, []int{2, 1}, false},
		{`{"id":1,"password":"value"}`, map[string]int{"id": 1}, false},
	} {
		fake := makeFake(t)
		apitest.Make(fake, jsonHandler(http.StatusOK, test.body)).Get("/").Do().JSONEqual(test.expected)
		if equal := len(fake.failures()) == 0; equal != test.equal {
			t.Errorf("JSONEqual(%s, %#v) equal = %v, want %v", test.body, test.expected, equal, test.equal)
		}
	}
}

func TestJSONEqualStopsOnAmbiguousDocuments(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		body, reason string
	}{
		{"", "body is empty"},
		{" \n", "body is empty"},
		{"null null", "unexpected data after the JSON document"},
		{`{"a":1,"a":2}`, `duplicate object key "a"`},
		{`[{"a":1,"\u0061":2}]`, `duplicate object key "a"`},
		{`{"a":`, "unexpected EOF"},
		{`{"a":1,}`, "(offset 7)"},
	} {
		response := apitest.Make(makeFake(t), jsonHandler(http.StatusOK, test.body)).Get("/").Do()
		message := expectFatal(t, func() { response.JSONEqual(map[string]int{"a": 1}) })
		expectContains(t, message, "not one unambiguous JSON document", test.reason)
	}
}

func TestJSONDecodesLikeAClient(t *testing.T) {
	t.Parallel()

	type item struct {
		ID int `json:"id"`
	}

	respond := func(body string) *apitest.Response {
		return apitest.Make(makeFake(t), jsonHandler(http.StatusOK, body)).Get("/").Do()
	}

	if got := respond(`{"id":1,"extra":true}`).JSON[item](); got.ID != 1 {
		t.Errorf("unknown field: id = %d, want 1", got.ID)
	}

	if got := respond(`{"id":1,"id":2}`).JSON[item](); got.ID != 2 {
		t.Errorf("duplicate key: id = %d, want the last value 2", got.ID)
	}

	if got := respond(`[1,2]`).JSON[[]int](); !slices.Equal(got, []int{1, 2}) {
		t.Errorf("array = %v, want [1 2]", got)
	}

	wrongType := respond(`{"id":"1"}`)
	expectContains(t, expectFatal(t, func() { wrongType.JSON[item]() }), "cannot unmarshal string", `"id": "1"`)
	empty := respond("")
	expectContains(t, expectFatal(t, func() { empty.JSON[item]() }), "unexpected end of JSON input", "<empty>")
}

func TestHeaderAssertions(t *testing.T) {
	t.Parallel()

	fake := makeFake(t)
	response := apitest.Make(fake, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Cache-Control", "no-store")
		w.Header().Add("Cache-Control", "private")
		w.Header().Set("X-Request-ID", "abc")
	})).Get("/").Do()

	response.
		Header("Cache-Control", "no-store", "private").
		HeaderAbsent("X-Missing").
		Header("Cache-Control", "no-store").
		HeaderAbsent("X-Request-ID").
		Header("X-Missing", "value")

	failures := fake.failures()
	if len(failures) != 3 {
		t.Fatalf("failures = %q, want three", failures)
	}

	expectContains(t, failures[0], `header "Cache-Control": expected "no-store", got ["no-store" "private"]`)
	expectContains(t, failures[1], `header "X-Request-ID": expected absence, got "abc"`)
	expectContains(t, failures[2], `header "X-Missing": expected "value", got <absent>`)
	expectContains(t, expectFatal(t, func() { response.Header("X-Request-ID") }), "use HeaderAbsent")
}

func TestBodyDiagnosticsAreBounded(t *testing.T) {
	t.Parallel()

	fake := makeFake(t)
	api := apitest.Make(fake, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/large" {
			_, _ = io.WriteString(w, strings.Repeat("x", 20000))
		}
	}))

	api.Get("/empty").Do().BodyEqual("expected")
	api.Get("/large").Do().BodyEqual("")

	failures := fake.failures()
	if len(failures) != 2 {
		t.Fatalf("failures = %q, want two", failures)
	}

	expectContains(t, failures[0], "expected:\nexpected\nactual:\n<empty>")
	expectContains(t, failures[1], "expected:\n<empty>", "… [11808 more bytes]")
	if len(failures[1]) > 8400 {
		t.Fatalf("diagnostic length = %d, want it bounded", len(failures[1]))
	}
}

func TestFailurePointsAtUserAssertion(t *testing.T) {
	if os.Getenv("APITEST_DIAGNOSTIC_CHILD") == "1" {
		response := apitest.Make(t, jsonHandler(http.StatusOK, "{}")).Get("/items").Do()
		_, _, line, _ := runtime.Caller(0)
		t.Logf("assertion-line=%d", line+2)
		response.JSONEqual(map[string]int{"id": 1})

		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestFailurePointsAtUserAssertion$", "-test.v")
	command.Env = append(os.Environ(), "APITEST_DIAGNOSTIC_CHILD=1")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("child assertion did not fail")
	}

	text := string(output)
	marker := strings.Index(text, "assertion-line=")
	if marker < 0 {
		t.Fatalf("missing source marker:\n%s", text)
	}

	line := strings.Fields(text[marker+len("assertion-line="):])[0]
	if _, err := strconv.Atoi(line); err != nil {
		t.Fatalf("invalid line marker:\n%s", text)
	}

	if !strings.Contains(text, "apitest_test.go:"+line+": GET /items") || strings.Contains(text, "response.go:") {
		t.Fatalf("failure does not point at the user's assertion:\n%s", text)
	}
}
