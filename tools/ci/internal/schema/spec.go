// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The formats project.v1 and host-settings.v1 (03 3.2, 3.4), whose Go
// types and rules live in internal/spec; tools/ci reaches them through
// internal/cmd/julieta/all (REQ-0971). Each schema carries the keywords
// of its rules, and each page its field table and its rule table
// (REQ-0172, REQ-1107).
func init() {
	project := schemagen.Schema{Name: "project.v1", Type: reflect.TypeFor[all.SpecProject](),
		Keywords: keywords(all.SpecSchemaKeywords(all.SpecFormatProject))}
	host := schemagen.Schema{Name: "host-settings.v1", Type: reflect.TypeFor[all.SpecHostSettings](),
		Keywords: keywords(all.SpecSchemaKeywords(all.SpecFormatHostSettings))}
	schemagen.Register(project)
	schemagen.Register(host)
	RegisterPage(Page{
		Format: project, Path: "docs/reference/project.md", Title: "Project spec",
		Reader: "a person who writes a project spec, or a contributor who changes its Go types or rules",
		Source: "internal/spec (the Go types, their gate and apply tags, and rules.go)",
		Intro: "The format `project.v1` of `projects/<name>.yaml` in a config repo, one project per file\n" +
			"([03 3.2](../spec/03-formats.md#32-project-spec-projectsnameyaml-schema-projectv1)).\n" +
			"Gate and Apply are the struct tags of a top-level field (01 1.4): a field whose gate is\n" +
			"widening is in the widening set that gate 2 asks for, and apply says when a change takes\n" +
			"effect, at the next create of the sandbox or on the running one. A member has the class\n" +
			"of its top-level field.",
		Tags:  []Tag{{Header: "Gate", Key: "gate"}, {Header: "Apply", Key: "apply"}},
		Check: func() error { _, err := all.SpecClassify(); return err },
		After: ungatedText(),
		Rules: ruleRows(all.SpecFormatProject),
	})
	RegisterPage(Page{
		Format: host, Path: "docs/reference/host-settings.md", Title: "Host settings",
		Reader: "a person who sets up romeu on a machine, or a contributor who changes the Go types or rules of host settings",
		Source: "internal/spec (the Go types and rules.go)",
		Intro: "The format `host-settings.v1` of `settings.yaml`, which romeu reads on the host\n" +
			"([03 3.4](../spec/03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)).\n" +
			"`romeu init` writes `root`, `config` and `tools`; the operator fills the rest.",
		Rules: ruleRows(all.SpecFormatHostSettings),
	})
}

// keywords returns the keywords of internal/spec as tools/schemagen's.
func keywords(in []all.SpecSchemaKeyword) []schemagen.Keyword {
	out := make([]schemagen.Keyword, len(in))
	for i, k := range in {
		out[i] = schemagen.Keyword{Path: k.Path, Values: k.Values}
	}
	return out
}

// ungatedText returns the text after the project's field table: the
// committed list of the ungated members (I7). The page's Check fails
// the generator when the tags and the list disagree, so the page is
// never written with a member on neither.
func ungatedText() string {
	var names []string
	for _, n := range all.SpecUngated() {
		names = append(names, "`"+n+"`")
	}
	return "The members deliberately left ungated, the committed list of 01 1.4 (I7): " + strings.Join(names, ", ") + "."
}

// ruleRows returns the rule table of format f.
func ruleRows(f all.SpecFormat) []RuleRow {
	var out []RuleRow
	for _, r := range all.SpecRules() {
		if r.Format != f {
			continue
		}
		e := "invalid document, exit 2"
		if r.Row != nil {
			e = fmt.Sprintf("`%s` (`%s`), exit %d", r.Row.ID, r.Row.Slug, r.Row.Exit)
		}
		out = append(out, RuleRow{Field: r.Field, Rule: describe(r), Message: r.Message, Error: e, Scope: string(r.Scope), Source: r.Source})
	}
	return out
}

// describe says the constraints of r in words, for the rule table.
func describe(r all.SpecRule) string {
	var parts []string
	if r.Pattern != "" {
		parts = append(parts, "matches `"+r.Pattern+"`")
	}
	if r.NotPattern != "" {
		parts = append(parts, "matches no `"+r.NotPattern+"`")
	}
	if r.MaxBytes != 0 {
		parts = append(parts, "at most "+strconv.Itoa(r.MaxBytes)+" bytes")
	}
	if len(r.Reserved) > 0 {
		parts = append(parts, "not "+codeList(r.Reserved, ", ")+" in any case")
	}
	if len(r.Enum) > 0 {
		parts = append(parts, "one of "+codeList(r.Enum, ", "))
	}
	if r.Range {
		parts = append(parts, fmt.Sprintf("from %g to %g", r.Min, r.Max))
	}
	if r.MinItems > 0 {
		parts = append(parts, fmt.Sprintf("at least %d element", r.MinItems))
	}
	if len(r.OneOf) > 0 {
		var groups []string
		for _, g := range r.OneOf {
			groups = append(groups, codeList(g, " and "))
		}
		parts = append(parts, "exactly one of "+strings.Join(groups, "; or "))
	}
	if r.Text != "" {
		parts = append(parts, r.Text)
	}
	return strings.Join(parts, "; ")
}

// codeList returns each of list in a code span, joined by sep.
func codeList(list []string, sep string) string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = "`" + s + "`"
	}
	return strings.Join(out, sep)
}
