/*
Package request decodes a JSON request body into a typed value.

[DecodeJSON] reads exactly one JSON document into T:

	type createArticle struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	}

	input, err := request.DecodeJSON[createArticle](r)

It is strict by default, because a body a client got wrong should fail rather
than apply in part:

  - the media type must be application/json or application/*+json;
  - an object field that T does not have is rejected, which catches typos;
    [AllowUnknownFields] turns this off;
  - anything after the first document is rejected.

# Errors

A problem with what the client sent is a [*DecodeError]. Its [DecodeErrorKind]
tells an empty body, an unsupported media type, malformed JSON, a value of the
wrong type, an invalid value and several documents apart. For a value of the
wrong type Path names the field, such as author.name. The cause is available
through errors.Unwrap for logging and is not meant for the client.

Any other failure, such as an error reading the body, is returned as is.
DecodeJSON imposes no size limit: wrap the body with http.MaxBytesReader, and
its *http.MaxBytesError reaches the caller unchanged, so the client gets 413
rather than 400.

The problem package maps both to Problem Details.

# Validation

[DecodeAndValidateJSON] decodes the body and then passes *T to a [Validator].
The interface has a single method, so any validation library fits behind a
one-line adapter. Its error is wrapped and stays available through errors.AsType.
*/
package request
