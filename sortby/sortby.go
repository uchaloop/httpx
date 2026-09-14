package sortby

import (
	"fmt"
	"strings"
)

// Direction is a supported sort direction.
type Direction string

const (
	Ascending  Direction = "asc"
	Descending Direction = "desc"
)

// Term describes one validated sort criterion.
type Term[Field any] struct {
	Field     Field
	Direction Direction
}

// Order is an ordered collection of validated sort criteria.
type Order[Field any] []Term[Field]

// Make converts criteria to application-owned values, preserving their order.
func (order Order[Field]) Make[Value any](makeTerm func(Field, Direction) Value) []Value {
	values := make([]Value, len(order))
	for index, term := range order {
		values[index] = makeTerm(term.Field, term.Direction)
	}

	return values
}

// Parse validates expressions using parseField. An expression may
// be either "field" or "field:direction". A missing direction means ascending.
// A ParseError reports the position of the rejected expression in Index.
func Parse[Field any](expressions []string, parseField func(string) (Field, error)) (Order[Field], error) {
	if parseField == nil {
		return nil, fmt.Errorf("sort field parser is nil")
	}

	order := make(Order[Field], 0, len(expressions))
	for index, expression := range expressions {
		term, err := parseExpression(index, expression)
		if err != nil {
			return nil, err
		}

		field, err := parseField(term.Field)
		if err != nil {
			return nil, &ParseError{Kind: ErrorUnknownField, Index: index, Expression: expression, Field: term.Field, Direction: string(term.Direction), cause: err}
		}

		order = append(order, Term[Field]{Field: field, Direction: term.Direction})
	}

	return order, nil
}

// ParseExpression parses one sort expression without restricting its field.
// Field validation may be applied by a framework adapter or application enum.
// A ParseError reports Index 0.
func ParseExpression(expression string) (Term[string], error) {
	return parseExpression(0, expression)
}

func parseExpression(index int, expression string) (Term[string], error) {
	if len(expression) == 0 {
		return Term[string]{}, makeParseError(ErrorEmptyExpression, index, expression, "", "")
	}

	if strings.Count(expression, ":") > 1 {
		return Term[string]{}, makeParseError(ErrorInvalidExpression, index, expression, "", "")
	}

	field, directionValue, hasDirection := strings.Cut(expression, ":")
	if len(field) == 0 {
		return Term[string]{}, makeParseError(ErrorEmptyField, index, expression, field, directionValue)
	}

	direction := Ascending
	if hasDirection {
		direction = Direction(directionValue)
		if direction != Ascending && direction != Descending {
			return Term[string]{}, makeParseError(
				ErrorUnknownDirection,
				index,
				expression,
				field,
				directionValue,
			)
		}
	}

	return Term[string]{Field: field, Direction: direction}, nil
}
