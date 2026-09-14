package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/uchaloop/httpx/page"
)

// ContentTypeJSON is the media type of a JSON response body.
const ContentTypeJSON = "application/json"

// Response is what a handler answers, before anything is written. A framework
// adapter writes it with the framework's own serializer; Write is the net/http
// backend.
type Response struct {
	Status int
	// Location is written as the Location header. Status 201 requires it.
	Location string
	// Header holds any other headers, such as a custom X-User; Content-Type and
	// Location have fields of their own. Names are canonicalized when written.
	Header http.Header
	// ContentType is the JSON media type of Body, and is empty for a response
	// without a body.
	ContentType string
	Body        any
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type pageData[Item any] struct {
	Items []Item `json:"items"`
	Total uint64 `json:"total"`
	Page  uint64 `json:"page"`
	Size  uint64 `json:"size"`
}

// JSON answers value with status.
func JSON(status int, value any) Response {
	return Response{Status: status, ContentType: ContentTypeJSON, Body: value}
}

// Data answers value inside a {"data": ...} envelope. A nil value is written
// explicitly as {"data":null}; it never changes the status.
func Data(status int, value any) Response {
	return JSON(status, dataEnvelope{Data: value})
}

// OK answers a data envelope with status 200, including for a nil value.
func OK(value any) Response {
	return Data(http.StatusOK, value)
}

// Created answers a data envelope with status 201 and a Location header, which
// must not be empty.
func Created(location string, value any) Response {
	resp := Data(http.StatusCreated, value)
	resp.Location = location

	return resp
}

// NoContent answers status 204 without a body or content type.
func NoContent() Response {
	return Response{Status: http.StatusNoContent}
}

// Page answers one page of items in a data envelope together with the total
// and the page position. Nil items are written as an empty array.
func Page[Item any](items []Item, total uint64, current page.Page) Response {
	if items == nil {
		items = make([]Item, 0)
	}

	return OK(pageData[Item]{Items: items, Total: total, Page: current.Number(), Size: current.Size()})
}

// Validate reports a response that must not be written: a status outside
// 200-599, a body with a status that forbids one, a body without a content
// type, or status 201 without a Location.
func (r Response) Validate() error {
	switch {
	case r.Status < http.StatusOK || r.Status > 599:
		return fmt.Errorf("status must be between 200 and 599, got %d", r.Status)
	case len(r.ContentType) > 0 && (r.Status == http.StatusNoContent || r.Status == http.StatusNotModified):
		return fmt.Errorf("status %d does not permit a response body", r.Status)
	case len(r.ContentType) == 0 && r.Body != nil:
		return fmt.Errorf("response body has no content type")
	case r.Status == http.StatusCreated && len(r.Location) == 0:
		return fmt.Errorf("status 201 requires a location")
	default:
		return nil
	}
}

// Write is the net/http backend. It encodes the body with encoding/json before
// committing anything, so an invalid response or an encoding failure leaves w
// untouched.
func Write(w http.ResponseWriter, resp Response) error {
	if err := resp.Validate(); err != nil {
		return err
	}

	var body []byte
	if len(resp.ContentType) > 0 {
		var err error
		if body, err = json.Marshal(resp.Body); err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
	}

	resp.SetHeaders(w.Header())
	w.WriteHeader(resp.Status)

	if len(resp.ContentType) == 0 {
		return nil
	}

	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}

	return nil
}

// SetHeaders writes the response headers into h: Header under canonical names,
// replacing values already set, then Location and Content-Type. Every backend
// uses it, so all adapters send the same headers.
func (r Response) SetHeaders(h http.Header) {
	for name, values := range r.Header {
		h[http.CanonicalHeaderKey(name)] = slices.Clone(values)
	}

	if len(r.Location) > 0 {
		h.Set("Location", r.Location)
	}

	if len(r.ContentType) > 0 {
		h.Set("Content-Type", r.ContentType)
	}
}
