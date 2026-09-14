/*
Package apitest tests an http.Handler through a real HTTP client and an
in-memory server.

A test states a request and what a client must see:

	func TestGetArticle(t *testing.T) {
		api := apitest.Make(t, handler)

		api.Get("/articles/42").Do().
			Status(http.StatusOK).
			Header("Content-Type", "application/json").
			JSONEqual(map[string]any{
				"data": map[string]any{"id": 42, "title": "HTTP boundaries"},
			})
	}

The request crosses a real HTTP connection, so routing, the headers net/http
adds, the status and the bytes of the body are what a client gets in
production. There is no network: [Make] starts a server from
httptest.NewTestServer and stops it when the test ends. A panic in the handler
fails the test.

# Requests

[Test] starts a request with Get, Post, Put, Patch, Delete, Head or Options, or
with any method through [Test.Request]. A [Request] sets headers, query
parameters, cookies, the Host, a context and at most one body: [Request.JSON]
encodes a value, and [Request.Body] sends raw bytes with any media type, for
example malformed JSON. Nothing is sent before [Request.Do], which reads and
closes the whole response.

The client follows no redirects and keeps no cookies, so every response is the
handler's own answer. A request fails after five seconds, or the duration set
with [WithTimeout], and always before the deadline of the test, so a handler
that hangs fails with the request in the message.

# Assertions

A [Response] checks the status, the headers and the body:

  - [Response.Status] stops the test on a mismatch and prints the body, because
    a response with another status has another shape;
  - [Response.Header] and [Response.HeaderAbsent] check header values in order;
  - [Response.BodyEqual] compares the exact body;
  - [Response.JSONEqual] compares the body with a value encoded by
    encoding/json.

JSONEqual ignores object key order and whitespace, and nothing else. Extra and
missing fields, array order and number literals count, so 1 and 1.0 differ as
they do for a typed client. A body with a duplicate object key fails, because
parsers disagree on which value wins. A mismatch lists up to ten differences by
path, such as $.data.items[0].title.

[Response.JSON] decodes the body into a type for checks that need code, such as
a generated identifier, and [Response.Raw] returns the buffered *http.Response.

# Parallel tests

A Test reports through the testing.TB it was made with. Make one for each
parallel subtest, so that a failure points at the subtest and its line.
*/
package apitest
