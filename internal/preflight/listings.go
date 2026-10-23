// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// maxListing is the most output of an sbx listing a parser reads, as
// for sbx ls --json (REQ-0271).
const maxListing = 1 << 20

// The shapes of the two listings of I22. The sessions of block A (probe
// A9) have not recorded them yet: both are assumed from the reference of
// sbx and from daily use, and the synthetic fixtures under
// testdata/synthetic hold them until A9 re-records them (rule 10 of
// docs/plan.md).
//
// The secret listing is a table of three columns under the header
//
//	SCOPE SERVICE SECRET
//
// one row per secret, the scope being "global" or "sandbox:<name>" and
// the secret column a masked value or a placeholder, which is never
// read. The policy listing is the array of sbx policy ls --json, each
// element an object with the required members scope (the same values),
// kind ("rule" or "default"), effect ("allow" or "deny") and resource
// (the pattern of a rule, empty for a default); other members are
// skipped.
const secretHeader = "SCOPE SERVICE SECRET"

// scopeOf reads a scope: "sandbox:<name>" with one name is a per-sandbox
// scope; every other value, "global" among them, counts as global, since
// a row whose scope is not exactly one sandbox may widen every sandbox
// (I22).
func scopeOf(scope string) (sandbox string, one bool) {
	name, ok := strings.CutPrefix(scope, "sandbox:")
	if !ok || name == "" || strings.ContainsAny(name, " \t\r\n,*:") {
		return "", false
	}
	return name, true
}

// secretRow is a row of the secret listing.
type secretRow struct {
	scope   string
	service string
}

// parseSecrets reads the secret listing. An output that is not the table
// of the recorded shape, a JSON document or an empty output among them,
// is upstream-shape (RJ-310), never an empty list.
func parseSecrets(out []byte) ([]secretRow, error) {
	const what = "secret listing"
	if len(out) > maxListing {
		return nil, upstreamShape(what, fmt.Sprintf("the output is longer than %d bytes", maxListing))
	}
	if !isText(out) {
		return nil, upstreamShape(what, "the output is not text")
	}
	var rows []secretRow
	header := false
	for n, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if !header {
			if strings.Join(fields, " ") != secretHeader {
				return nil, upstreamShape(what, "the first line is not the header "+secretHeader)
			}
			header = true
			continue
		}
		if len(fields) != 3 {
			return nil, upstreamShape(what, fmt.Sprintf("line %d does not have three columns", n+1))
		}
		rows = append(rows, secretRow{scope: fields[0], service: fields[1]})
	}
	if !header {
		return nil, upstreamShape(what, "the output is empty")
	}
	return rows, nil
}

// policyRow is a row of the policy listing.
type policyRow struct {
	scope    string
	kind     string
	effect   string
	resource string
}

// wirePolicy is a row as sbx writes it; a member that is read is a
// pointer, so a missing one and a null one are both nil.
type wirePolicy struct {
	Scope    *string `json:"scope"`
	Kind     *string `json:"kind"`
	Effect   *string `json:"effect"`
	Resource *string `json:"resource"`
}

// parsePolicies reads the policy listing: a JSON array of rows.
// Unknown members are skipped; a duplicate member name, a missing or
// mistyped member that is read, a kind or an effect of a value that is
// not recorded, a document cut short or a top level that is not an array
// is upstream-shape (RJ-310), never an empty list: only [] is empty.
func parsePolicies(out []byte) ([]policyRow, error) {
	const what = "policy ls --json"
	if len(out) > maxListing {
		return nil, upstreamShape(what, fmt.Sprintf("the output is longer than %d bytes", maxListing))
	}
	if first := bytes.TrimLeft(out, " \t\r\n"); len(first) == 0 || first[0] != '[' {
		return nil, upstreamShape(what, "the document is not an array of policies")
	}
	var wire []*wirePolicy
	if err := jsonv2.Unmarshal(out, &wire); err != nil {
		return nil, upstreamShape(what, describe(err))
	}
	rows := make([]policyRow, 0, len(wire))
	for i, w := range wire {
		if w == nil || w.Scope == nil || w.Kind == nil || w.Effect == nil || w.Resource == nil {
			return nil, upstreamShape(what, fmt.Sprintf("policy %d lacks a member that is read, or has it null", i+1))
		}
		if *w.Kind != "rule" && *w.Kind != "default" || *w.Effect != "allow" && *w.Effect != "deny" {
			return nil, upstreamShape(what, fmt.Sprintf("policy %d has a kind or an effect that is not recorded", i+1))
		}
		rows = append(rows, policyRow{scope: *w.Scope, kind: *w.Kind, effect: *w.Effect, resource: *w.Resource})
	}
	return rows, nil
}

// isText reports whether out is valid UTF-8 with no NUL.
func isText(out []byte) bool {
	return !bytes.ContainsRune(out, 0) && utf8.Valid(out)
}

// describe says what the decoder found, without the text of the output:
// the decoder's own message for a value that does not parse holds it.
func describe(err error) string {
	var syn *jsontext.SyntacticError
	var sem *jsonv2.SemanticError
	switch {
	case errors.As(err, &syn):
		return "the document is not JSON, or is cut short, or repeats a member name"
	case errors.As(err, &sem):
		return "a member has the wrong type or a value that does not parse, at " + string(sem.JSONPointer)
	}
	return "the document does not decode"
}

// upstreamShape returns the error upstream-shape for the output of the
// sbx call what: it names sbx and the recorded versions (REQ-0064,
// REQ-0170).
func upstreamShape(what, why string) *cli.Error {
	return cli.UpstreamShape.New("the output of sbx "+what+" is not a shape romeu reads: "+why,
		cli.Detail{Key: "upstream", Value: "sbx"},
		cli.Detail{Key: "recorded sbx versions", Value: strings.Join(sbxdrv.RecordedVersions, ", ")})
}
