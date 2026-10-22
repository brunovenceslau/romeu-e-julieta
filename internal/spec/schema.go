// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

// SchemaKeyword is a JSON Schema keyword the generated schema of a format
// carries at the place Path names, in the path syntax of Rule.Field.
type SchemaKeyword struct {
	Path   string
	Values map[string]any
}

// SchemaKeywords returns the constraints of the rules of format f that
// JSON Schema can say, in the order of Rules, for tools/schemagen
// (REQ-0172): a pattern, a pattern no value matches, a length, an enum,
// a range, the fewest elements, and the groups of members of OneOf. A
// reserved value, a Value check and the checks across values are the
// validators' alone, so a document the schema accepts can still be
// refused by romeu or julieta, never the other way round. MaxBytes is
// written as maxLength, which counts characters, so the schema's bound
// is the looser one.
func SchemaKeywords(f Format) []SchemaKeyword {
	var out []SchemaKeyword
	for _, r := range rulesOf(f) {
		v := map[string]any{}
		if r.Pattern != "" {
			v["pattern"] = r.Pattern
		}
		if r.NotPattern != "" {
			v["not"] = map[string]any{"pattern": r.NotPattern}
		}
		if r.MaxBytes != 0 {
			v["maxLength"] = r.MaxBytes
		}
		if len(r.Enum) > 0 {
			v["enum"] = r.Enum
		}
		if r.Range {
			v["minimum"], v["maximum"] = r.Min, r.Max
		}
		if r.MinItems > 0 {
			v["minItems"] = r.MinItems
		}
		if len(r.OneOf) > 0 {
			v["oneOf"] = oneOfSchemas(r.OneOf)
		}
		if len(v) > 0 && r.inSet == nil {
			out = append(out, SchemaKeyword{Path: r.Field, Values: v})
		}
	}
	return out
}

// oneOfSchemas returns the oneOf of groups: for each group, its members
// required and no member of another group present.
func oneOfSchemas(groups [][]string) []any {
	var out []any
	for i, g := range groups {
		var others []any
		for j, o := range groups {
			if j != i {
				for _, m := range o {
					others = append(others, map[string]any{"required": []any{m}})
				}
			}
		}
		req := make([]any, len(g))
		for k, m := range g {
			req[k] = m
		}
		out = append(out, map[string]any{"required": req, "not": map[string]any{"anyOf": others}})
	}
	return out
}
