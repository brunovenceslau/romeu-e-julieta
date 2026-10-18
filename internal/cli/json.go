// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
)

// jsonError is the --json form of an error (04 4.1, Error ids, item 4):
// details is the object the text form prints after the first line, and
// the "recovered in" cell is not in it.
type jsonError struct {
	ID      string            `json:"id"`
	Slug    string            `json:"slug"`
	Exit    int               `json:"exit"`
	Message string            `json:"message"`
	Fix     string            `json:"fix"`
	Details map[string]string `json:"details"`
}

// writeJSONError writes e as one JSON object on one line to w. Text that
// is not UTF-8 becomes U+FFFD rather than failing the error report.
func writeJSONError(w io.Writer, e *Error) {
	details := make(map[string]string, len(e.Details))
	for _, d := range e.Details {
		details[d.Key] = d.Value
	}
	doc := jsonError{ID: e.ID, Slug: e.Slug, Exit: e.Exit, Message: e.Error(), Fix: e.Hint(), Details: details}
	b, err := json.Marshal(doc, json.Deterministic(true), jsontext.AllowInvalidUTF8(true))
	if err != nil {
		// A struct of strings and an int always marshals; this keeps the
		// report from vanishing if that ever changes.
		b = []byte(`{"id":"` + e.ID + `","slug":"` + e.Slug + `"}`)
	}
	_, _ = w.Write(append(b, '\n'))
}
