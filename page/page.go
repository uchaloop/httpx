package page

import "math"

// Params contains optional client-provided pagination parameters. A nil value
// means that the parameter was not provided; a pointer to zero remains an
// explicitly invalid value.
type Params struct {
	Number *uint64
	Size   *uint64
}

// Config defines the pagination policy chosen by an application endpoint.
type Config struct {
	DefaultSize uint64
	MaxSize     uint64
	// MaxOffset is an optional downstream offset limit. Zero disables it.
	MaxOffset uint64
}

// Page contains validated pagination values.
type Page struct {
	number uint64
	size   uint64
	offset uint64
}

// Make validates params against cfg and calculates a page.
func Make(params Params, cfg Config) (Page, error) {
	if cfg.DefaultSize == 0 {
		return Page{}, makeError(ErrorInvalidDefaultSize, 0, 0)
	}

	if cfg.MaxSize == 0 {
		return Page{}, makeError(ErrorInvalidMaxSize, 0, 0)
	}

	if cfg.DefaultSize > cfg.MaxSize {
		return Page{}, makeError(
			ErrorDefaultSizeExceedsMaximum,
			cfg.DefaultSize,
			cfg.MaxSize,
		)
	}

	number := uint64(1)
	if params.Number != nil {
		number = *params.Number
	}

	if number == 0 {
		return Page{}, makeError(ErrorInvalidNumber, number, 0)
	}

	size := cfg.DefaultSize
	if params.Size != nil {
		size = *params.Size
	}

	if size == 0 {
		return Page{}, makeError(ErrorInvalidSize, size, 0)
	}

	if size > cfg.MaxSize {
		return Page{}, makeError(ErrorSizeExceedsMaximum, size, cfg.MaxSize)
	}

	// Offset limits are reported as page numbers, the value a client controls.
	// Neither maximum below overflows: size 1 never exceeds either bound.
	previousPages := number - 1
	if maxPreviousPages := math.MaxUint64 / size; previousPages > maxPreviousPages {
		return Page{}, makeError(ErrorOffsetOverflow, number, maxPreviousPages+1)
	}

	offset := previousPages * size
	if cfg.MaxOffset > 0 && offset > cfg.MaxOffset {
		return Page{}, makeError(ErrorOffsetExceedsMaximum, number, cfg.MaxOffset/size+1)
	}

	return Page{
		number: number,
		size:   size,
		offset: offset,
	}, nil
}

// Number returns the one-based page number.
func (p Page) Number() uint64 {
	return p.number
}

// Size returns the requested number of items per page.
func (p Page) Size() uint64 {
	return p.size
}

// Offset returns the zero-based item offset.
func (p Page) Offset() uint64 {
	return p.offset
}

// Limit returns the requested number of items per page.
func (p Page) Limit() uint64 {
	return p.size
}
