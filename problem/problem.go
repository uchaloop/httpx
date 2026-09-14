package problem

import (
	"net/http"

	"github.com/uchaloop/httpx/response"
)

const (
	// ContentType is the RFC 9457 Problem Details JSON media type.
	ContentType = "application/problem+json"

	internalErrorCode = "internal_error"
	aboutBlankType    = "about:blank"
)

// InvalidParam describes one invalid request value.
type InvalidParam struct {
	Path    string `json:"path"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// Problem is an RFC 9457 Problem Details document with stable code and errors
// extension members.
type Problem struct {
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Status   int            `json:"status"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	Code     string         `json:"code,omitempty"`
	Errors   []InvalidParam `json:"errors,omitempty"`

	// Header holds response headers the problem calls for, such as Accept for
	// 415 or WWW-Authenticate for 401. It is sent, but is not part of the
	// document.
	Header http.Header `json:"-"`
}

// Template describes the public parts of a problem before a request-specific
// instance is added by Mapper.
type Template struct {
	Type   string
	Title  string
	Status int
	Detail string
	Code   string
	Errors []InvalidParam
	Header http.Header
}

// MakeProblem builds the problem tmpl describes for r with the defaults a
// Mapper applies: about:blank as the type, the status text as the title, and
// the request path as the instance. It does not validate tmpl.
func MakeProblem(r *http.Request, tmpl Template) Problem {
	return makeProblem(tmpl, requestInstance(r))
}

// Response answers the problem as application/problem+json with its status.
func (p Problem) Response() response.Response {
	return response.Response{Status: p.Status, ContentType: ContentType, Header: p.Header, Body: p}
}

func makeProblem(tmpl Template, instance string) Problem {
	problemType := tmpl.Type
	if len(problemType) == 0 {
		problemType = aboutBlankType
	}

	title := tmpl.Title
	if len(title) == 0 {
		title = http.StatusText(tmpl.Status)
	}

	return Problem{
		Type:     problemType,
		Title:    title,
		Status:   tmpl.Status,
		Detail:   tmpl.Detail,
		Instance: instance,
		Code:     tmpl.Code,
		Errors:   cloneInvalidParams(tmpl.Errors),
		Header:   tmpl.Header.Clone(),
	}
}

func makeInternalProblem(instance string) Problem {
	return makeProblem(Template{
		Type:   aboutBlankType,
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Code:   internalErrorCode,
	}, instance)
}

func cloneInvalidParams(params []InvalidParam) []InvalidParam {
	if params == nil {
		return nil
	}

	cloned := make([]InvalidParam, len(params))
	copy(cloned, params)

	return cloned
}
