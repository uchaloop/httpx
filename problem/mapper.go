package problem

import (
	"errors"
	"fmt"
	"net/http"
)

// Rule maps a matching error to a public problem template.
//
// Rules are created by WhenIs, WhenAs, or When. Mapper evaluates them in the
// order supplied to MakeMapper and uses the first match.
type Rule struct {
	apply     func(error) (Template, bool)
	configErr error
}

// MakeMapper creates an ordered error mapper. It returns an error when a rule
// is not configured correctly.
func MakeMapper(rules ...Rule) (*Mapper, error) {
	for index, rule := range rules {
		if rule.configErr != nil {
			return nil, fmt.Errorf("rule %d: %w", index, rule.configErr)
		}

		if rule.apply == nil {
			return nil, fmt.Errorf("rule %d has no matcher", index)
		}
	}

	copied := make([]Rule, len(rules))
	copy(copied, rules)

	return &Mapper{rules: copied}, nil
}

// Mapper converts errors to request-specific Problem Details documents.
type Mapper struct {
	rules []Rule
}

// Map applies the first matching rule. Unknown errors and invalid templates
// become a safe internal problem that does not expose the source error.
func (m *Mapper) Map(r *http.Request, err error) Problem {
	instance := requestInstance(r)
	if mapped, matched := m.mapKnown(instance, err); matched {
		return mapped
	}

	return makeInternalProblem(instance)
}

// MapKnown applies the first matching rule and reports whether a rule matched.
// It allows transport adapters to apply framework-native fallbacks only when
// the application mapper has not classified an error explicitly.
func (m *Mapper) MapKnown(r *http.Request, err error) (Problem, bool) {
	return m.mapKnown(requestInstance(r), err)
}

func (m *Mapper) mapKnown(instance string, err error) (Problem, bool) {
	if m != nil {
		for _, rule := range m.rules {
			template, matched := rule.apply(err)
			if !matched {
				continue
			}

			if err := validateTemplate(template); err != nil {
				return makeInternalProblem(instance), true
			}

			return makeProblem(template, instance), true
		}
	}

	return Problem{}, false
}

// WhenIs creates a rule using errors.Is. It therefore matches both target and
// errors that wrap target.
func WhenIs(target error, tmpl Template) Rule {
	if target == nil {
		return Rule{configErr: fmt.Errorf("errors.Is target is nil")}
	}

	if err := validateTemplate(tmpl); err != nil {
		return Rule{configErr: err}
	}

	return Rule{apply: func(err error) (Template, bool) {
		return tmpl, errors.Is(err, target)
	}}
}

// WhenAs creates a rule using errors.AsType. makeTemplate receives the matched
// typed error and may derive safe public details from it.
func WhenAs[E error](makeTemplate func(E) Template) Rule {
	if makeTemplate == nil {
		return Rule{configErr: fmt.Errorf("errors.AsType template function is nil")}
	}

	return Rule{apply: func(err error) (Template, bool) {
		target, ok := errors.AsType[E](err)
		if !ok {
			return Template{}, false
		}

		return makeTemplate(target), true
	}}
}

// When creates a rule for application-specific classification. The callback
// returns a template and true when it handled err.
func When(match func(error) (Template, bool)) Rule {
	if match == nil {
		return Rule{configErr: fmt.Errorf("match function is nil")}
	}

	return Rule{apply: match}
}

func validateTemplate(tmpl Template) error {
	if tmpl.Status < http.StatusBadRequest || tmpl.Status > 599 {
		return fmt.Errorf("problem status must be between 400 and 599, got %d", tmpl.Status)
	}

	if len(tmpl.Title) == 0 && len(http.StatusText(tmpl.Status)) == 0 {
		return fmt.Errorf("problem title is required for non-standard status %d", tmpl.Status)
	}

	return nil
}

func requestInstance(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}

	path := r.URL.EscapedPath()
	if len(path) == 0 {
		return "/"
	}

	return path
}
