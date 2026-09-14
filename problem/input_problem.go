package problem

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/uchaloop/httpx/page"
	"github.com/uchaloop/httpx/request"
	"github.com/uchaloop/httpx/sortby"
)

// InputProblemRule maps errors produced by httpx request, page, and sortby
// packages to stable RFC 9457 templates. Error codes use go-playground/validator
// tag names where the constraint has one (min, max) and the enum tag for closed
// sets; a JSON value of the wrong type has code type. Invalid server-side page
// configuration remains an internal error.
func InputProblemRule() Rule {
	return When(func(err error) (Template, bool) {
		if decodeErr, ok := errors.AsType[*request.DecodeError](err); ok {
			return jsonProblemTemplate(decodeErr), true
		}

		// http.MaxBytesReader reports a body over its limit while it is read.
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return Template{Status: http.StatusRequestEntityTooLarge}, true
		}

		if pageErr, ok := errors.AsType[*page.Error](err); ok {
			return pageProblemTemplate(pageErr)
		}

		if sortErr, ok := errors.AsType[*sortby.ParseError](err); ok {
			// Any rejected expression is outside the closed set of field and
			// direction pairs accepted by the endpoint. The message names the
			// element as go-playground/validator names a slice element.
			index := strconv.Itoa(sortErr.Index)
			return InvalidRequest(http.StatusUnprocessableEntity, InvalidParam{
				Path:    "sort." + index,
				Code:    "enum",
				Message: "sort[" + index + "] must be one of the allowed values",
			}), true
		}

		return Template{}, false
	})
}

func jsonProblemTemplate(err *request.DecodeError) Template {
	// A body in a media type the endpoint does not read is 415, with the Accept
	// header RFC 9110 names for it. Like other status-only transport problems,
	// it has no code.
	if err.Kind == request.DecodeErrorUnsupportedMediaType {
		return Template{
			Status: http.StatusUnsupportedMediaType,
			Header: http.Header{"Accept": {"application/json"}},
		}
	}

	template := Template{
		Status: http.StatusBadRequest,
		Detail: "Request body must contain one valid JSON document",
		Code:   "invalid_json",
	}

	if err.Kind == request.DecodeErrorWrongType && len(err.Path) > 0 {
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			template.Errors = []InvalidParam{TypeParam(err.Path, typeErr.Type)}
		}
	}

	return template
}

func pageProblemTemplate(err *page.Error) (Template, bool) {
	var param InvalidParam
	switch err.Kind {
	case page.ErrorInvalidNumber:
		param = minParam("page", 1)
	case page.ErrorInvalidSize:
		param = minParam("size", 1)
	case page.ErrorSizeExceedsMaximum:
		param = maxParam("size", err.Limit)
	case page.ErrorOffsetOverflow, page.ErrorOffsetExceedsMaximum:
		param = maxParam("page", err.Limit)
	default:
		return Template{}, false
	}

	return InvalidRequest(http.StatusUnprocessableEntity, param), true
}
