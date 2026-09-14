package request

import "fmt"

// DecodeErrorKind identifies a stable class of request JSON decoding failure.
type DecodeErrorKind uint8

const (
	DecodeErrorEmptyBody DecodeErrorKind = iota + 1
	DecodeErrorUnsupportedMediaType
	DecodeErrorMalformedJSON
	DecodeErrorWrongType
	DecodeErrorInvalidValue
	DecodeErrorMultipleValues
)

// DecodeError describes a recoverable client input error. Cause remains
// available through errors.Unwrap for detailed mapping and logging.
type DecodeError struct {
	Kind  DecodeErrorKind
	Path  string
	cause error
}

func (e *DecodeError) Error() string {
	if e == nil {
		return "JSON decode error"
	}

	message := decodeErrorMessage(e.Kind)
	if len(e.Path) > 0 {
		message += fmt.Sprintf(" at %s", e.Path)
	}

	if e.cause != nil {
		message += ": " + e.cause.Error()
	}

	return message
}

// Unwrap returns the underlying decoder or reader error.
func (e *DecodeError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.cause
}

func makeDecodeError(kind DecodeErrorKind, path string, cause error) *DecodeError {
	return &DecodeError{Kind: kind, Path: path, cause: cause}
}

func decodeErrorMessage(kind DecodeErrorKind) string {
	switch kind {
	case DecodeErrorEmptyBody:
		return "JSON body is empty"
	case DecodeErrorUnsupportedMediaType:
		return "unsupported JSON media type"
	case DecodeErrorMalformedJSON:
		return "malformed JSON"
	case DecodeErrorWrongType:
		return "wrong JSON value type"
	case DecodeErrorInvalidValue:
		return "invalid JSON value"
	case DecodeErrorMultipleValues:
		return "multiple JSON values"
	default:
		return "JSON decode error"
	}
}
