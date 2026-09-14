package apitest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
)

const (
	// maxDifferences bounds the reported differences, not the comparison.
	maxDifferences = 10
	valueLimit     = 256
)

// decodeJSON decodes exactly one document. It keeps number literals and
// rejects duplicate object keys, which JSON parsers resolve differently, so an
// exact comparison of such a document proves nothing.
func decodeJSON(body []byte) (any, error) {
	if len(bytes.Trim(body, " \t\r\n")) == 0 {
		return nil, errors.New("body is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	value, err := decodeValue(decoder)
	if err != nil {
		if syntaxErr, ok := errors.AsType[*json.SyntaxError](err); ok {
			return nil, fmt.Errorf("%w (offset %d)", err, syntaxErr.Offset)
		}

		return nil, err
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unexpected data after the JSON document (offset %d)", decoder.InputOffset())
	}

	return value, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	token, err := nextToken(dec)
	if err != nil {
		return nil, err
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}

	switch delimiter {
	case '{':
		object := map[string]any{}
		for dec.More() {
			key, err := nextToken(dec)
			if err != nil {
				return nil, err
			}

			name := key.(string)
			if _, exists := object[name]; exists {
				return nil, fmt.Errorf("duplicate object key %q", name)
			}

			if object[name], err = decodeValue(dec); err != nil {
				return nil, err
			}
		}

		if _, err := nextToken(dec); err != nil {
			return nil, err
		}

		return object, nil
	case '[':
		array := []any{}
		for dec.More() {
			element, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}

			array = append(array, element)
		}

		if _, err := nextToken(dec); err != nil {
			return nil, err
		}

		return array, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}

// nextToken reports the end of input inside a document as truncation.
func nextToken(dec *json.Decoder) (json.Token, error) {
	token, err := dec.Token()
	if errors.Is(err, io.EOF) {
		return nil, io.ErrUnexpectedEOF
	}

	return token, err
}

// jsonDiff collects differences between decoded documents. Scalars, including
// json.Number literals, are compared exactly.
type jsonDiff struct {
	lines []string
	total int
}

func diffJSON(want, got any) []string {
	var diff jsonDiff
	diff.compare("$", want, got)
	if hidden := diff.total - len(diff.lines); hidden > 0 {
		diff.lines = append(diff.lines, fmt.Sprintf("… and %d more", hidden))
	}

	return diff.lines
}

func (d *jsonDiff) compare(path string, want, got any) {
	switch expected := want.(type) {
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok {
			d.mismatch(path, want, got)

			return
		}

		for _, key := range unionKeys(expected, actual) {
			child := path + jsonKey(key)
			wantValue, wanted := expected[key]
			gotValue, present := actual[key]
			switch {
			case !present:
				d.add(child, "missing field, expected %s", formatJSON(wantValue))
			case !wanted:
				d.add(child, "unexpected field, got %s", formatJSON(gotValue))
			default:
				d.compare(child, wantValue, gotValue)
			}
		}
	case []any:
		actual, ok := got.([]any)
		if !ok {
			d.mismatch(path, want, got)

			return
		}

		for index := range max(len(expected), len(actual)) {
			child := path + "[" + strconv.Itoa(index) + "]"
			switch {
			case index >= len(actual):
				d.add(child, "missing element, expected %s", formatJSON(expected[index]))
			case index >= len(expected):
				d.add(child, "unexpected element, got %s", formatJSON(actual[index]))
			default:
				d.compare(child, expected[index], actual[index])
			}
		}
	default:
		if want != got {
			d.mismatch(path, want, got)
		}
	}
}

func (d *jsonDiff) mismatch(path string, want, got any) {
	if wantKind, gotKind := jsonKind(want), jsonKind(got); wantKind != gotKind {
		d.add(path, "expected %s %s, got %s %s", wantKind, formatJSON(want), gotKind, formatJSON(got))

		return
	}

	d.add(path, "expected %s, got %s", formatJSON(want), formatJSON(got))
}

func (d *jsonDiff) add(path, format string, args ...any) {
	d.total++
	if len(d.lines) < maxDifferences {
		d.lines = append(d.lines, path+": "+fmt.Sprintf(format, args...))
	}
}

func unionKeys(want, got map[string]any) []string {
	keys := slices.Collect(maps.Keys(want))
	for key := range got {
		if _, ok := want[key]; !ok {
			keys = append(keys, key)
		}
	}

	slices.Sort(keys)

	return keys
}

// jsonKey writes identifier-like keys as .key and quotes the rest.
func jsonKey(key string) string {
	for index, char := range key {
		letter := char == '_' || 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z'
		if !letter && (index == 0 || char < '0' || char > '9') {
			return "[" + strconv.Quote(key) + "]"
		}
	}

	if len(key) == 0 {
		return `[""]`
	}

	return "." + key
}

func jsonKind(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	default:
		return "null"
	}
}

func formatJSON(value any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Sprint(value)
	}

	return clip(strings.TrimSuffix(buffer.String(), "\n"), valueLimit)
}

func joinLines(lines []string) string {
	return "  " + strings.Join(lines, "\n  ")
}
