package sortby

import "fmt"

// ErrorKind identifies a stable sort expression failure class.
type ErrorKind uint8

const (
	ErrorEmptyExpression ErrorKind = iota + 1
	ErrorInvalidExpression
	ErrorUnknownField
	ErrorUnknownDirection
	ErrorEmptyField
)

// ParseError describes an invalid sort expression. Index is the position of
// the expression among the values passed to Parse.
type ParseError struct {
	Kind       ErrorKind
	Index      int
	Expression string
	Field      string
	Direction  string
	cause      error
}

// Unwrap preserves the application field parser's error.
func (e *ParseError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.cause
}

func (e *ParseError) Error() string {
	if e == nil {
		return "invalid sort expression"
	}

	switch e.Kind {
	case ErrorEmptyExpression:
		return "sort expression is empty"
	case ErrorInvalidExpression:
		return fmt.Sprintf("sort expression %q has too many separators", e.Expression)
	case ErrorUnknownField:
		return fmt.Sprintf("sort field %q is not allowed", e.Field)
	case ErrorUnknownDirection:
		return fmt.Sprintf("sort direction %q is not supported", e.Direction)
	case ErrorEmptyField:
		return "sort field is empty"
	default:
		return "invalid sort expression"
	}
}

func makeParseError(
	kind ErrorKind,
	index int,
	expression string,
	field string,
	direction string,
) *ParseError {
	return &ParseError{
		Kind:       kind,
		Index:      index,
		Expression: expression,
		Field:      field,
		Direction:  direction,
	}
}
