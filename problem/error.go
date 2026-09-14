package problem

import (
	"errors"
	"net/http"
)

// Error carries a public problem template at the HTTP transport boundary.
// Domain and service packages should continue to return domain errors.
type Error struct {
	template Template
	cause    error
}

// MakeError creates a transport error with an optional underlying cause.
// ErrorRule maps it without exposing cause in the response.
func MakeError(tmpl Template, cause error) *Error {
	return &Error{template: cloneTemplate(tmpl), cause: cause}
}

// MakeInvalidRequest creates the standard invalid_request transport error.
func MakeInvalidRequest(status int, cause error, params ...InvalidParam) *Error {
	return MakeError(InvalidRequest(status, params...), cause)
}

// InvalidRequest returns the standard invalid_request template, also used by
// framework adapters for their binding and validation errors. Status 400 means
// request values could not be decoded; 422 means decoded values violate a
// constraint.
func InvalidRequest(status int, params ...InvalidParam) Template {
	detail := "Request parameters are invalid"
	if status == http.StatusBadRequest {
		detail = "Request parameters could not be decoded"
	}

	return Template{
		Status: status,
		Detail: detail,
		Code:   "invalid_request",
		Errors: params,
	}
}

func (e *Error) Error() string {
	if e == nil {
		return "HTTP problem"
	}

	if e.cause != nil {
		return e.cause.Error()
	}

	if len(e.template.Detail) > 0 {
		return e.template.Detail
	}

	if len(e.template.Title) > 0 {
		return e.template.Title
	}

	if title := http.StatusText(e.template.Status); len(title) > 0 {
		return title
	}

	return "HTTP problem"
}

// Unwrap returns the transport operation's underlying error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.cause
}

// ErrorRule maps errors created by MakeError. It is normally placed with the
// common transport rules before domain-specific rules.
func ErrorRule() Rule {
	return When(func(err error) (Template, bool) {
		problemErr, ok := errors.AsType[*Error](err)
		if !ok || problemErr == nil {
			return Template{}, false
		}

		return cloneTemplate(problemErr.template), true
	})
}

func cloneTemplate(tmpl Template) Template {
	tmpl.Errors = cloneInvalidParams(tmpl.Errors)
	tmpl.Header = tmpl.Header.Clone()

	return tmpl
}
