package sortby_test

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/uchaloop/httpx/sortby"
)

type column string

const (
	columnTitle     column = "title"
	columnUpdatedAt column = "updated_at"
)

// parseColumn allows only the fields the endpoint can sort by.
func parseColumn(name string) (column, error) {
	switch name {
	case "title":
		return columnTitle, nil
	case "updatedAt":
		return columnUpdatedAt, nil
	default:
		return "", fmt.Errorf("unknown sort field %q", name)
	}
}

func ExampleParse() {
	order, err := sortby.Parse([]string{"updatedAt:desc", "title"}, parseColumn)
	if err != nil {
		log.Fatal(err)
	}

	for _, term := range order {
		fmt.Println(term.Field, term.Direction)
	}
	// Output:
	// updated_at desc
	// title asc
}

func ExampleOrder_Make() {
	order, err := sortby.Parse([]string{"updatedAt:desc", "title"}, parseColumn)
	if err != nil {
		log.Fatal(err)
	}

	clauses := order.Make(func(field column, direction sortby.Direction) string {
		return string(field) + " " + strings.ToUpper(string(direction))
	})

	fmt.Println("ORDER BY", strings.Join(clauses, ", "))
	// Output: ORDER BY updated_at DESC, title ASC
}

func ExampleParseError() {
	for _, expressions := range [][]string{{"title", "author"}, {"title:up"}} {
		_, err := sortby.Parse(expressions, parseColumn)
		if parseErr, ok := errors.AsType[*sortby.ParseError](err); ok {
			fmt.Printf("sort[%d]: %v\n", parseErr.Index, err)
		}
	}
	// Output:
	// sort[1]: sort field "author" is not allowed
	// sort[0]: sort direction "up" is not supported
}

func ExampleParseExpression() {
	term, err := sortby.ParseExpression("createdAt:desc")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(term.Field, term.Direction)
	// Output: createdAt desc
}
