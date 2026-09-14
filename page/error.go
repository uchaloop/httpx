package page

import "fmt"

// ErrorKind identifies a stable pagination failure class.
type ErrorKind uint8

const (
	ErrorInvalidDefaultSize ErrorKind = iota + 1
	ErrorInvalidMaxSize
	ErrorDefaultSizeExceedsMaximum
	ErrorInvalidNumber
	ErrorInvalidSize
	ErrorSizeExceedsMaximum
	ErrorOffsetOverflow
	ErrorOffsetExceedsMaximum
)

// Error describes an invalid pagination policy or parameter. Value is the
// rejected value and Limit the bound it violates. For offset errors both are
// page numbers: the requested page and the largest page allowed.
type Error struct {
	Kind  ErrorKind
	Value uint64
	Limit uint64
}

func (e *Error) Error() string {
	if e == nil {
		return "invalid pagination"
	}

	switch e.Kind {
	case ErrorInvalidDefaultSize:
		return "default page size must be greater than zero"
	case ErrorInvalidMaxSize:
		return "maximum page size must be greater than zero"
	case ErrorDefaultSizeExceedsMaximum:
		return fmt.Sprintf(
			"default page size %d exceeds maximum %d",
			e.Value,
			e.Limit,
		)
	case ErrorInvalidNumber:
		return "page number must be greater than zero"
	case ErrorInvalidSize:
		return "page size must be greater than zero"
	case ErrorSizeExceedsMaximum:
		return fmt.Sprintf("page size %d exceeds maximum %d", e.Value, e.Limit)
	case ErrorOffsetOverflow:
		return fmt.Sprintf(
			"page number %d exceeds maximum %d: offset overflows",
			e.Value,
			e.Limit,
		)
	case ErrorOffsetExceedsMaximum:
		return fmt.Sprintf(
			"page number %d exceeds maximum %d allowed by the offset limit",
			e.Value,
			e.Limit,
		)
	default:
		return "invalid pagination"
	}
}

func makeError(kind ErrorKind, value uint64, limit uint64) *Error {
	return &Error{Kind: kind, Value: value, Limit: limit}
}
