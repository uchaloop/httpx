package sortby_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/uchaloop/httpx/sortby"
)

func parseField(value string) (string, error) {
	switch value {
	case "id", "createdAt", "name":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported field %q", value)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		expressions []string
		want        sortby.Order[string]
	}{
		{name: "no sort", want: sortby.Order[string]{}},
		{
			name:        "directions",
			expressions: []string{"createdAt:desc", "name", "id:asc"},
			want: sortby.Order[string]{
				{Field: "createdAt", Direction: sortby.Descending},
				{Field: "name", Direction: sortby.Ascending},
				{Field: "id", Direction: sortby.Ascending},
			},
		},
	} {
		got, err := sortby.Parse(test.expressions, parseField)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: Parse = %#v, %v; want %#v", test.name, got, err, test.want)
		}
	}
}

func TestParseRejectsInvalidExpressions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		expression string
		kind       sortby.ErrorKind
	}{
		{name: "empty", expression: "", kind: sortby.ErrorEmptyExpression},
		{name: "too many separators", expression: "id:asc:extra", kind: sortby.ErrorInvalidExpression},
		{name: "missing field", expression: ":asc", kind: sortby.ErrorEmptyField},
		{name: "unknown direction", expression: "id:sideways", kind: sortby.ErrorUnknownDirection},
		{name: "empty direction", expression: "id:", kind: sortby.ErrorUnknownDirection},
		{name: "unknown field", expression: "secret:asc", kind: sortby.ErrorUnknownField},
		{name: "untrimmed field", expression: " id", kind: sortby.ErrorUnknownField},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// The rejected expression follows a valid one, so Index must name it.
			_, err := sortby.Parse([]string{"id", test.expression}, parseField)
			parseErr, ok := errors.AsType[*sortby.ParseError](err)
			if !ok {
				t.Fatalf("error = %v, want *sortby.ParseError", err)
			}

			if parseErr.Kind != test.kind || parseErr.Index != 1 || parseErr.Expression != test.expression {
				t.Errorf("error = %+v, want kind %d at index 1 for %q", parseErr, test.kind, test.expression)
			}
		})
	}
}

func TestParseStopsAtFirstRejectedExpression(t *testing.T) {
	t.Parallel()

	cause := errors.New("field unavailable")
	var calls []string
	order, err := sortby.Parse([]string{"id", "secret:desc", "name"}, func(value string) (string, error) {
		calls = append(calls, value)
		if value == "secret" {
			return "", cause
		}

		return value, nil
	})

	parsed, ok := errors.AsType[*sortby.ParseError](err)
	if order != nil || !errors.Is(err, cause) || !ok {
		t.Fatalf("order = %v, error = %v; want no partial order and the parser's cause", order, err)
	}

	if parsed.Field != "secret" || parsed.Direction != "desc" {
		t.Errorf("error context = %+v", parsed)
	}

	if !reflect.DeepEqual(calls, []string{"id", "secret"}) {
		t.Errorf("parser calls = %v, want parsing to stop at the rejected field", calls)
	}
}

func TestParseChecksSyntaxBeforeCallingFieldParser(t *testing.T) {
	t.Parallel()

	_, err := sortby.Parse([]string{"id:sideways"}, func(string) (string, error) {
		t.Fatal("field parser called for invalid syntax")
		return "", nil
	})

	if parseErr, ok := errors.AsType[*sortby.ParseError](err); !ok || parseErr.Kind != sortby.ErrorUnknownDirection {
		t.Fatalf("error = %v, want an unknown direction", err)
	}
}

func TestParseExpressionLeavesFieldValidationToConsumer(t *testing.T) {
	t.Parallel()

	got, err := sortby.ParseExpression("custom:desc")
	if err != nil || got != (sortby.Term[string]{Field: "custom", Direction: sortby.Descending}) {
		t.Fatalf("ParseExpression = %+v, %v; want custom descending", got, err)
	}

	_, err = sortby.ParseExpression("custom:sideways")
	if parseErr, ok := errors.AsType[*sortby.ParseError](err); !ok || parseErr.Kind != sortby.ErrorUnknownDirection {
		t.Fatalf("error = %v, want the direction still validated", err)
	}
}

func TestOrderMakePreservesOrderAndDirection(t *testing.T) {
	t.Parallel()

	type field uint8
	type criterion struct {
		Field      field
		Descending bool
	}

	order, err := sortby.Parse([]string{"id:desc", "id"}, func(value string) (field, error) {
		if value != "id" {
			return 0, errors.New("unsupported field")
		}

		return 1, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got := order.Make(func(f field, direction sortby.Direction) criterion {
		return criterion{Field: f, Descending: direction == sortby.Descending}
	})
	if !reflect.DeepEqual(got, []criterion{{Field: 1, Descending: true}, {Field: 1}}) {
		t.Fatalf("criteria = %+v", got)
	}
}

func TestParseRejectsNilFieldParser(t *testing.T) {
	t.Parallel()

	if _, err := sortby.Parse[string](nil, nil); err == nil {
		t.Fatal("nil parser accepted")
	}
}
