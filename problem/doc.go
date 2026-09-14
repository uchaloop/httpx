/*
Package problem answers errors as RFC 9457 Problem Details.

A [Problem] is the document a client receives, with the application/problem+json
media type:

	{
	  "type": "about:blank",
	  "title": "Not Found",
	  "status": 404,
	  "detail": "Article was not found",
	  "instance": "/articles/42",
	  "code": "article_not_found"
	}

Besides the members defined by the RFC it has two extensions: code, a stable
identifier a client can branch on, and errors, a list of [InvalidParam] values
that name each rejected request value by its path.

# Mapping errors

Services return their own errors; the transport decides what a client sees. A
[Mapper] holds an ordered list of rules and turns an error into a Problem for
the request, with the request path as the instance:

	mapper, err := problem.MakeMapper(
		problem.WhenIs(article.ErrNotFound, problem.Template{
			Status: http.StatusNotFound,
			Code:   "article_not_found",
			Detail: "Article was not found",
		}),
		problem.InputProblemRule(),
		problem.ErrorRule(),
	)

[WhenIs] matches with errors.Is, so wrapped errors match too. [WhenAs] matches
with errors.AsType and builds the template from the typed error. [When] runs any
classification. The first matching rule wins.

A [Template] holds only what is public. An empty type becomes about:blank and
an empty title becomes the status text. [MakeMapper] rejects a rule that is not
configured correctly, such as a status outside 400-599, when the application
starts rather than when the error happens.

[Mapper.Map] never exposes an error it does not know. An error no rule matches,
or a rule that returns an invalid template, becomes status 500 with code
internal_error and nothing else. Log the error; the client sees only the
problem. [Mapper.MapKnown] reports whether a rule matched, so a router adapter
can fall back to its own answer, such as its 404 or 405, only for errors the
application did not classify.

# Input errors

[InputProblemRule] maps the errors of the request, page and sortby packages:

  - a body that is not one JSON document: 400 with code invalid_json, and the
    path of a value of the wrong type in errors;
  - a body in another media type: 415 with an Accept: application/json header;
  - a body over an http.MaxBytesReader limit: 413;
  - a page number or size out of range: 422 with code invalid_request;
  - a sort expression outside the allowed set: 422 with code invalid_request.

A handler that rejects a value itself returns [MakeInvalidRequest], or any
template with [MakeError], and [ErrorRule] maps it. The cause stays available
for logging and never reaches the client. [InvalidRequest] is the template of
the standard invalid_request problem: status 400 means that request values could
not be decoded, 422 that decoded values break a constraint.

# Codes and messages

The code of an [InvalidParam] follows go-playground/validator: a constraint is
named by its validation tag, such as required, min or max, and the message
follows the tag's English translation, such as "size must be 100 or less". A
value rejected here reads the same as one rejected by a validator. Two codes
have no tag: type for a value that cannot be decoded into its field
([TypeParam]), and enum for a value outside a closed set.

# Writing

[Problem.Response] answers the problem as a response.Response, so anything that
writes a response writes a problem. [Write] writes it through net/http.

The Header field of a Problem carries headers the problem calls for, such as
Accept for 415, WWW-Authenticate for 401 or Retry-After for 429. They are sent
with the response and are not part of the document. [MakeProblem] builds a
problem from a template without a mapper, for example for a router's own
status errors.
*/
package problem
