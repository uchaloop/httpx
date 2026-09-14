package page_test

import (
	"errors"
	"math"
	"testing"

	"github.com/uchaloop/httpx/page"
)

func pointer(value uint64) *uint64 {
	return &value
}

func TestMakeCalculatesPage(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                 string
		params               page.Params
		config               page.Config
		number, size, offset uint64
	}{
		{name: "defaults", config: page.Config{DefaultSize: 10, MaxSize: 100}, number: 1, size: 10},
		{
			name:   "second page",
			params: page.Params{Number: pointer(2), Size: pointer(25)},
			config: page.Config{DefaultSize: 10, MaxSize: 100},
			number: 2, size: 25, offset: 25,
		},
		{
			name:   "maximum size",
			params: page.Params{Size: pointer(100)},
			config: page.Config{DefaultSize: 10, MaxSize: 100},
			number: 1, size: 100,
		},
		{
			name:   "offset at its maximum",
			params: page.Params{Number: pointer(3)},
			config: page.Config{DefaultSize: 10, MaxSize: 100, MaxOffset: 20},
			number: 3, size: 10, offset: 20,
		},
		{
			name:   "no offset maximum",
			params: page.Params{Number: pointer(101)},
			config: page.Config{DefaultSize: 10, MaxSize: 100},
			number: 101, size: 10, offset: 1000,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := page.Make(test.params, test.config)
			if err != nil {
				t.Fatalf("Make: %v", err)
			}

			if got.Number() != test.number || got.Size() != test.size || got.Offset() != test.offset || got.Limit() != test.size {
				t.Errorf("page = number %d, size %d, offset %d, limit %d; want %d, %d, %d, %d",
					got.Number(), got.Size(), got.Offset(), got.Limit(), test.number, test.size, test.offset, test.size)
			}
		})
	}
}

func TestMakeRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		params       page.Params
		config       page.Config
		kind         page.ErrorKind
		value, limit uint64
	}{
		{name: "zero default size", config: page.Config{MaxSize: 100}, kind: page.ErrorInvalidDefaultSize},
		{name: "zero maximum size", config: page.Config{DefaultSize: 10}, kind: page.ErrorInvalidMaxSize},
		{
			name: "default exceeds maximum", config: page.Config{DefaultSize: 101, MaxSize: 100},
			kind: page.ErrorDefaultSizeExceedsMaximum, value: 101, limit: 100,
		},
		{
			name: "zero number", params: page.Params{Number: pointer(0)}, config: page.Config{DefaultSize: 10, MaxSize: 100},
			kind: page.ErrorInvalidNumber,
		},
		{
			name: "zero size", params: page.Params{Size: pointer(0)}, config: page.Config{DefaultSize: 10, MaxSize: 100},
			kind: page.ErrorInvalidSize,
		},
		{
			name: "size over maximum", params: page.Params{Size: pointer(101)}, config: page.Config{DefaultSize: 10, MaxSize: 100},
			kind: page.ErrorSizeExceedsMaximum, value: 101, limit: 100,
		},
		// Offset limits are reported in page numbers, the value a client controls.
		{
			name: "offset over maximum", params: page.Params{Number: pointer(4)},
			config: page.Config{DefaultSize: 10, MaxSize: 100, MaxOffset: 20},
			kind:   page.ErrorOffsetExceedsMaximum, value: 4, limit: 3,
		},
		{
			name: "offset overflow", params: page.Params{Number: pointer(math.MaxUint64), Size: pointer(2)},
			config: page.Config{DefaultSize: 1, MaxSize: 2},
			kind:   page.ErrorOffsetOverflow, value: math.MaxUint64, limit: math.MaxUint64/2 + 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := page.Make(test.params, test.config)
			pageErr, ok := errors.AsType[*page.Error](err)
			if !ok {
				t.Fatalf("error = %v, want *page.Error", err)
			}

			if pageErr.Kind != test.kind || pageErr.Value != test.value || pageErr.Limit != test.limit {
				t.Errorf("error = kind %d, value %d, limit %d; want %d, %d, %d",
					pageErr.Kind, pageErr.Value, pageErr.Limit, test.kind, test.value, test.limit)
			}
		})
	}
}
