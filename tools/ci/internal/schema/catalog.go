// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The format catalog.v1 (03 3.5), whose Go type lives in
// internal/catalog; tools/ci reaches it through internal/cmd/julieta/all
// (REQ-0971). The schema carries the pattern of a tool key; the domain
// rule (REQ-0188) is the readers' alone.
func init() {
	s := schemagen.Schema{Name: "catalog.v1", Type: reflect.TypeFor[all.CatalogFile](),
		Keywords: []schemagen.Keyword{{Path: "tools{key}", Values: map[string]any{"pattern": all.CatalogToolKeyPattern}}}}
	schemagen.Register(s)
	const domain = "lowercase host name, IDNA A-labels, no wildcard, port or IP literal"
	RegisterPage(Page{
		Format: s, Path: "docs/reference/catalog.md", Title: "Egress catalog",
		Reader: "a contributor who adds a tool or a domain to the egress catalog",
		Source: "internal/catalog (the Go types)",
		Intro: "The format `catalog.v1` of `internal/catalog/egress/*.yaml`, one file per tool family,\n" +
			"embedded in romeu and julieta ([03 3.5](../spec/03-formats.md#35-egress-catalog-internalcatalogegressyaml-schema-catalogv1)).\n" +
			"A file is written by the maintainer, so its reader refuses an unknown key (ADR 0041). The\n" +
			"`meta` and `base` sets live in `base.yaml` alone. A domain or a tool that two files declare\n" +
			"with different entries is a decode error, and `tools/ci catalog` refuses a domain used\n" +
			"anywhere without an `upload` entry.",
		Rules: []RuleRow{
			{Field: "meta[]", Rule: domain, Message: "is not an egress domain", Error: "invalid document, exit 2", Scope: "both", Source: "REQ-0188"},
			{Field: "base[]", Rule: domain, Message: "is not an egress domain", Error: "invalid document, exit 2", Scope: "both", Source: "REQ-0188"},
			{Field: "domains{key}", Rule: domain, Message: "is not an egress domain", Error: "invalid document, exit 2", Scope: "both", Source: "REQ-0188"},
			{Field: "tools{key}", Rule: "matches `" + all.CatalogToolKeyPattern + "`", Message: "is not a backend:tool of mise.lock", Error: "invalid document, exit 2", Scope: "both", Source: "REQ-0842"},
			{Field: "tools{}[]", Rule: domain, Message: "is not an egress domain", Error: "invalid document, exit 2", Scope: "both", Source: "REQ-0188"},
		},
	})
}
