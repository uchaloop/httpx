<p align="center">
  <a href="https://github.com/uchaloop/httpx/actions/workflows/ci.yml"><img src="https://github.com/uchaloop/httpx/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/uchaloop/httpx"><img src="https://pkg.go.dev/badge/github.com/uchaloop/httpx.svg" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/uchaloop/httpx" alt="License: MIT"></a>
</p>

# httpx

Building blocks for JSON HTTP APIs in Go that work with any router. httpx gives
you responses as values, errors as [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457)
Problem Details, strict JSON request bodies, pagination and sorting parameters,
and tests that go through HTTP. It is not a framework: handlers, routing and
middleware stay yours.

- **Answers are values** - a handler returns a `response.Response`. net/http
  writes it with `response.Write`; a router writes it with its own serializer.
- **Errors do not leak** - every error reaches the client as Problem Details,
  and an error no rule knows becomes a 500 that says nothing about it.
- **The validator's vocabulary** - error codes and messages follow the tag names
  and English translations of go-playground/validator, so a value rejected by
  httpx reads like one rejected by a validator.
- **Strict input** - a body in another media type, with an unknown field or
  with a second document fails instead of applying in part.
- **Standard library only** - no dependencies, and a test keeps it that way.

```bash
go get github.com/uchaloop/httpx
```

httpx requires Go 1.27: `sortby` and `apitest` use generic methods.

## Quick start

A handler returns an answer or an error. One small function turns such handlers
into `http.Handler` values and maps every error through a `problem.Mapper`:

```go
var errDuplicateTitle = errors.New("duplicate title")

type article struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type createArticle struct {
	Title string `json:"title"`
}

func main() {
	mapper, err := problem.MakeMapper(
		problem.WhenIs(errDuplicateTitle, problem.Template{
			Status: http.StatusConflict,
			Code:   "duplicate_title",
			Detail: "An article with this title already exists",
		}),
		problem.InputProblemRule(),
		problem.ErrorRule(),
	)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /articles", handle(mapper, create))

	log.Fatal(http.ListenAndServe(":8080", mux))
}

func create(r *http.Request) (response.Response, error) {
	input, err := request.DecodeJSON[createArticle](r)
	if err != nil {
		return response.Response{}, err
	}

	created := article{ID: 42, Title: input.Title} // stored by a service

	return response.Created("/articles/"+strconv.FormatInt(created.ID, 10), created), nil
}

// handle adapts a handler that returns an answer to net/http.
func handle(m *problem.Mapper, h func(*http.Request) (response.Response, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		answer, err := h(r)
		if err != nil {
			// Log err here: the client gets only the mapped problem.
			answer = m.Map(r, err).Response()
		}

		if err := response.Write(w, answer); err != nil {
			log.Printf("write response: %v", err)
		}
	}
}
```

A valid request is created:

```http
POST /articles
Content-Type: application/json

{"title": "HTTP boundaries"}
```

```http
HTTP/1.1 201 Created
Content-Type: application/json
Location: /articles/42

{"data":{"id":42,"title":"HTTP boundaries"}}
```

A title that is not a string never reaches the service:

```http
HTTP/1.1 400 Bad Request
Content-Type: application/problem+json

{
  "type": "about:blank",
  "title": "Bad Request",
  "status": 400,
  "detail": "Request body must contain one valid JSON document",
  "instance": "/articles",
  "code": "invalid_json",
  "errors": [
    {"path": "title", "code": "type", "message": "title must be a string"}
  ]
}
```

## Packages

| Package    | What it does                                                        |
|------------|---------------------------------------------------------------------|
| `response` | A successful answer as a value, and its net/http backend            |
| `problem`  | RFC 9457 Problem Details and the mapping of errors to them          |
| `request`  | Strict decoding of a JSON request body                              |
| `page`     | Page number and size validated against the policy of an endpoint    |
| `sortby`   | Sort expressions parsed into a typed order                          |
| `apitest`  | Handler tests through a real HTTP client and an in-memory server    |

## Responses

```go
response.OK(article)                      // 200 {"data": article}
response.Created("/articles/42", article) // 201 {"data": article}, Location
response.NoContent()                      // 204, no body
response.JSON(http.StatusAccepted, job)   // any status, no envelope
response.Page(items, total, current)      // 200 {"data": {"items", "total", "page", "size"}}
```

`Page` writes nil items as `[]`, so a client never has to tell `null` from an
empty list.

Location and Content-Type have fields of their own; any other header goes to
`Header`. Names are canonicalized when written, so `x-user` and `X-User` are the
same header whichever way a handler spells it:

```go
answer := response.OK(profile)
answer.Header = http.Header{"x-user": {userID}} // sent as X-User
```

`Validate` rejects what must not be written: a status outside 200-599, a body
on 204 or 304, a body without a content type, 201 without a Location.
`response.Write` validates and encodes the body before it commits anything, so
after a failure the writer is untouched and the handler can still answer with an
error.

## Problem Details

A `problem.Mapper` holds ordered rules and turns an error into a problem for the
request. `WhenIs` matches with `errors.Is`, so wrapped errors match too;
`WhenAs` matches with `errors.AsType` and builds the problem from the typed error:

```go
mapper, err := problem.MakeMapper(
	problem.WhenIs(article.ErrNotFound, problem.Template{
		Status: http.StatusNotFound,
		Code:   "article_not_found",
		Detail: "Article was not found",
	}),
	problem.WhenAs(func(quota *QuotaError) problem.Template {
		return problem.Template{
			Status: http.StatusTooManyRequests,
			Code:   "quota_exceeded",
			Header: http.Header{"Retry-After": {quota.RetryAfter()}},
		}
	}),
	problem.InputProblemRule(),
	problem.ErrorRule(),
)
```

The first matching rule wins. `MakeMapper` checks the rules, such as a status
outside 400-599, when the application starts rather than when the error
happens. An error no rule matches becomes a problem that exposes nothing:

```json
{"type":"about:blank","title":"Internal Server Error","status":500,"instance":"/articles/42","code":"internal_error"}
```

`Header` carries headers a problem calls for - `Accept` on 415,
`WWW-Authenticate` on 401, `Retry-After` on 429. They are sent with the
response and are not part of the document. `MapKnown` reports whether a rule
matched, so a router can apply its own answer, such as its 404 or 405 built with
`problem.MakeProblem`, only to errors the application did not classify.

### Input errors

`InputProblemRule` maps the errors of `request`, `page` and `sortby`:

| Input                                                     | Status | `code`            | `errors[].code` |
|-----------------------------------------------------------|--------|-------------------|-----------------|
| Content-Type is not JSON                                  | 415, with `Accept: application/json` | | |
| Body over an `http.MaxBytesReader` limit                  | 413    |                   |                 |
| Empty body, malformed JSON, unknown field, two documents  | 400    | `invalid_json`    |                 |
| JSON value of the wrong type                              | 400    | `invalid_json`    | `type`          |
| Page number or size below 1                               | 422    | `invalid_request` | `min`           |
| Size or page number over the maximum                      | 422    | `invalid_request` | `max`           |
| Sort field or direction the endpoint does not allow       | 422    | `invalid_request` | `enum`          |

Status 400 means a value could not be decoded; 422 means a decoded value breaks
a constraint:

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "Request parameters are invalid",
  "instance": "/articles",
  "code": "invalid_request",
  "errors": [
    {"path": "size", "code": "max", "message": "size must be 100 or less"}
  ]
}
```

A handler that checks a value itself reports it the same way. `ErrorRule` maps
the error, and its cause stays available for logging only:

```go
return response.Response{}, problem.MakeInvalidRequest(http.StatusUnprocessableEntity, nil,
	problem.InvalidParam{Path: "title", Code: "required", Message: "title is a required field"})
```

### Codes and messages

`errors[].code` is the name of the go-playground/validator tag for the
constraint - `required`, `min`, `max`, `email` - and `message` follows the tag's
English translation. Where no tag exists, httpx uses two codes of its own:
`type` for a value that cannot be decoded into its field (`problem.TypeParam`),
and `enum` for a value outside a closed set.

## Request bodies

```go
input, err := request.DecodeJSON[createArticle](r)
```

`DecodeJSON` reads exactly one JSON document in `application/json` or
`application/*+json`. An unknown field is rejected unless
`request.AllowUnknownFields()` is passed. A failure is a `*request.DecodeError`
with a kind and, for a value of the wrong type, the path of the field.

`DecodeJSON` sets no size limit. Wrap the body, and the limit's error reaches
the mapper unchanged, so the client gets 413 rather than 400:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
```

`DecodeAndValidateJSON` passes the decoded `*T` to a `request.Validator`, an
interface with a single `Validate(any) error` method. Any validation library
fits behind a one-line adapter:

```go
type structValidator struct{ validate *validator.Validate }

func (v structValidator) Validate(value any) error { return v.validate.Struct(value) }
```

Turning the library's errors into `problem.InvalidParam` values - the field
path, the tag as the code, the translated message - belongs to the application
or to a router adapter.

## Pagination and sorting

```go
func list(r *http.Request) (response.Response, error) {
	current, err := page.Make(
		page.Params{Number: number, Size: size}, // nil when the client sent none
		page.Config{DefaultSize: 20, MaxSize: 100},
	)
	if err != nil {
		return response.Response{}, err // 422, code min or max
	}

	order, err := sortby.Parse(r.URL.Query()["sort"], parseColumn)
	if err != nil {
		return response.Response{}, err // 422, code enum at sort.N
	}

	items, total, err := store.List(r.Context(), order, current.Limit(), current.Offset())
	if err != nil {
		return response.Response{}, err
	}

	return response.Page(items, total, current), nil
}
```

`page.Config.MaxOffset` caps how deep a client may page; the error names the
last page allowed. A sort expression is `field` or `field:direction`, such as
`?sort=status&sort=updatedAt:desc`, and `parseColumn` decides which fields an
endpoint allows, so only those reach the storage. `Order.Make` converts the
order into a type of the storage package.

Reading `page` and `size` from the query is the router's binding job. A value
that is not a number is reported with `TypeParam`:

```go
number, err := strconv.ParseUint(r.URL.Query().Get("page"), 10, 64)
if err != nil {
	return response.Response{}, problem.MakeInvalidRequest(http.StatusBadRequest, err,
		problem.TypeParam("page", reflect.TypeFor[uint64]())) // page must be a non-negative integer
}
```

## Testing handlers

`apitest` sends a request through a real HTTP client to an in-memory server and
checks what the client gets:

```go
func TestCreateArticle(t *testing.T) {
	t.Parallel()

	api := apitest.Make(t, routes())

	api.Post("/articles").JSON(map[string]any{"title": "HTTP boundaries"}).Do().
		Status(http.StatusCreated).
		Header("Location", "/articles/42").
		JSONEqual(map[string]any{
			"data": map[string]any{"id": 42, "title": "HTTP boundaries"},
		})

	api.Post("/articles").Body("text/plain", strings.NewReader("title=HTTP")).Do().
		Status(http.StatusUnsupportedMediaType).
		Header("Accept", "application/json")
}
```

Routing, headers, status and the bytes of the body are what a client gets in
production, with no network. `JSONEqual` ignores key order and whitespace and
nothing else - extra fields, array order and number literals count - and a
mismatch lists the differences by path:

```text
POST /articles
JSON differs:
  $.data.title: expected "HTTP", got "HTTP boundaries"
body (application/json):
{
  "data": {
    "id": 42,
    "title": "HTTP boundaries"
  }
}
```

The client follows no redirects and keeps no cookies. Each request fails after
five seconds (`apitest.WithTimeout`), always before the test's own deadline, so
a handler that hangs fails with the request in the message. `Response.JSON[T]`
decodes the body for checks that need code, such as a generated identifier.

## Using a router

httpx does not depend on a router. net/http and routers built on its handlers
write with `response.Write` and `problem.Write`. A router with its own
serializer writes a `Response` in three steps:

```go
func write(ctx *router.Context, resp response.Response) error {
	if err := resp.Validate(); err != nil {
		return err
	}

	resp.SetHeaders(ctx.Response().Header())
	if len(resp.ContentType) == 0 {
		return ctx.NoContent(resp.Status)
	}

	return ctx.JSON(resp.Status, resp.Body)
}
```

The serializer must keep the Content-Type that `SetHeaders` sets, because a
problem is `application/problem+json`. Binding errors of the router become
`problem.InvalidRequest` templates with the same codes, so every router answers
the same input the same way.

## Documentation

Every package, type and function, with runnable examples:
**[pkg.go.dev/github.com/uchaloop/httpx](https://pkg.go.dev/github.com/uchaloop/httpx)**.

## Acknowledgements

I am grateful to the authors of [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457),
of [go-playground/validator](https://github.com/go-playground/validator), whose
tags and translations httpx speaks, and of [Laravel](https://laravel.com), whose
API resources, pagination and HTTP tests shaped these building blocks.

## License

[MIT](LICENSE)
