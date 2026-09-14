package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

type jsonOptions struct {
	allowUnknownFields bool
}

// JSONOption changes DecodeJSON behavior.
type JSONOption func(*jsonOptions)

// AllowUnknownFields allows object fields that are not present in the target
// Go type. DecodeJSON rejects them by default to catch client-side typos.
func AllowUnknownFields() JSONOption {
	return func(o *jsonOptions) {
		o.allowUnknownFields = true
	}
}

// DecodeJSON decodes one required JSON document from r.Body into T.
//
// The request must use application/json or an application/*+json media type.
// Unknown object fields and multiple JSON documents are rejected by default.
// DecodeJSON does not impose a body-size limit. A failure to read the body,
// such as an exceeded limit, is returned as is rather than as a DecodeError,
// so the limit's own status reaches the client.
func DecodeJSON[T any](r *http.Request, opts ...JSONOption) (T, error) {
	var value T

	if r == nil {
		return value, fmt.Errorf("HTTP request is nil")
	}

	if r.Body == nil {
		return value, makeDecodeError(DecodeErrorEmptyBody, "", io.EOF)
	}

	mediaType, err := requestMediaType(r)
	if err != nil {
		return value, makeDecodeError(DecodeErrorUnsupportedMediaType, "", err)
	}

	if !isJSONMediaType(mediaType) {
		return value, makeDecodeError(
			DecodeErrorUnsupportedMediaType,
			"",
			fmt.Errorf("Content-Type %q is not JSON", mediaType),
		)
	}

	config := jsonOptions{}
	for _, apply := range opts {
		if apply != nil {
			apply(&config)
		}
	}

	decoder := json.NewDecoder(bodyReader{reader: r.Body})
	if !config.allowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(&value); err != nil {
		if errors.Is(err, io.EOF) {
			return value, makeDecodeError(DecodeErrorEmptyBody, "", err)
		}

		return value, decodeFailure(err)
	}

	// One token is enough to tell the end of input from a second document, and
	// unlike Decode it neither allocates nor parses that document whole.
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return value, makeDecodeError(DecodeErrorMultipleValues, "", nil)
		}

		return value, decodeFailure(err)
	}

	return value, nil
}

// bodyReadError marks a failure of the body reader itself, which is not a
// problem with the JSON the client sent.
type bodyReadError struct {
	err error
}

func (e *bodyReadError) Error() string {
	return e.err.Error()
}

// bodyReader tags read failures. A body cut short keeps io.ErrUnexpectedEOF:
// it is truncated JSON from the client, reported as malformed.
type bodyReader struct {
	reader io.Reader
}

func (r bodyReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		err = &bodyReadError{err: err}
	}

	return count, err
}

func decodeFailure(err error) error {
	if readErr, ok := errors.AsType[*bodyReadError](err); ok {
		return fmt.Errorf("read request body: %w", readErr.err)
	}

	return decodeErrorFromJSONError(err)
}

func requestMediaType(r *http.Request) (string, error) {
	contentType := r.Header.Get("Content-Type")
	if len(contentType) == 0 {
		return "", fmt.Errorf("Content-Type header is required")
	}

	// Without parameters there is nothing to parse, and ParseMediaType would
	// still allocate a parameter map for every request. The media type is
	// compared case-insensitively.
	if !strings.Contains(contentType, ";") {
		return strings.TrimSpace(contentType), nil
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("parse Content-Type: %w", err)
	}

	return mediaType, nil
}

func isJSONMediaType(mediaType string) bool {
	if strings.EqualFold(mediaType, "application/json") {
		return true
	}

	lower := strings.ToLower(mediaType)

	return strings.HasPrefix(lower, "application/") &&
		strings.HasSuffix(lower, "+json") &&
		len(lower) > len("application/+json")
}

func decodeErrorFromJSONError(err error) *DecodeError {
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return makeDecodeError(DecodeErrorMalformedJSON, "", err)
	}

	if _, ok := errors.AsType[*json.SyntaxError](err); ok {
		return makeDecodeError(DecodeErrorMalformedJSON, "", err)
	}

	if typeError, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		return makeDecodeError(DecodeErrorWrongType, typeError.Field, err)
	}

	return makeDecodeError(DecodeErrorInvalidValue, "", err)
}
