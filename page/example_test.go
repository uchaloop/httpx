package page_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/uchaloop/httpx/page"
)

func ExampleMake() {
	number, size := uint64(3), uint64(20)

	current, err := page.Make(
		page.Params{Number: &number, Size: &size},
		page.Config{DefaultSize: 10, MaxSize: 100},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(current.Number(), current.Size(), current.Offset())
	// Output: 3 20 40
}

func ExampleMake_defaults() {
	// A client that sends nothing gets the first page of the default size.
	current, err := page.Make(page.Params{}, page.Config{DefaultSize: 10, MaxSize: 100})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(current.Number(), current.Size(), current.Offset())
	// Output: 1 10 0
}

func ExampleConfig_maxOffset() {
	// Storage that skips at most 1000 rows serves pages up to 51 of size 20.
	number := uint64(60)

	_, err := page.Make(
		page.Params{Number: &number},
		page.Config{DefaultSize: 20, MaxSize: 100, MaxOffset: 1000},
	)

	fmt.Println(err)
	// Output: page number 60 exceeds maximum 51 allowed by the offset limit
}

func ExampleError() {
	size := uint64(500)

	_, err := page.Make(page.Params{Size: &size}, page.Config{DefaultSize: 10, MaxSize: 100})
	if pageErr, ok := errors.AsType[*page.Error](err); ok && pageErr.Kind == page.ErrorSizeExceedsMaximum {
		fmt.Println(pageErr.Value, "is over", pageErr.Limit)
	}
	// Output: 500 is over 100
}
