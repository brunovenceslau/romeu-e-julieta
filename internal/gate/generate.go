// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"fmt"
	"go/format"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// GeneratedFile is one file the generator of this package writes: its
// slash-separated path from the root of the tree and its bytes.
type GeneratedFile struct {
	Path    string
	Content []byte
}

// The files Generated writes (12 12.3, REQ-1106).
const (
	// FieldsPath holds the field lists of the widening set and of the
	// recreate digest.
	FieldsPath = "internal/gate/fields_gen.go"
	// FlipTablePath holds the I7 flip table, one case per field.
	FlipTablePath = "internal/gate/flip_gen_test.go"
)

// Generated returns the consumers of the gate and apply tags of
// internal/spec that live in this package: the field lists and the I7
// flip table. It fails when spec.Classify fails, on a field that is
// neither tagged nor on the committed list of ungated fields (I7), and
// when a gated field has no member of the widening set or a member
// names a field that is not gated (setMembers), so a forgotten tag or
// mapping is a generator failure, not a silent strip.
func Generated() ([]GeneratedFile, error) { return generate(spec.Classify) }

// generate is Generated for the classification classify returns.
func generate(classify func() ([]spec.FieldClass, error)) ([]GeneratedFile, error) {
	classes, err := classify()
	if err != nil {
		return nil, fmt.Errorf("gate: generate: %w", err)
	}
	var widening, ungated, recreate []string
	for _, c := range classes {
		if c.Gate == spec.GateWidening {
			if _, ok := setMembers[c.Name]; !ok {
				return nil, fmt.Errorf("gate: generate: the field %s is tagged gate:%q and no member of the widening set holds it; add it to setMembers", c.Name, spec.GateWidening)
			}
			widening = append(widening, c.Name)
		} else {
			ungated = append(ungated, c.Name)
		}
		if c.Apply == spec.ApplyRecreate {
			recreate = append(recreate, c.Name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(setMembers)) {
		if !slices.Contains(widening, name) {
			return nil, fmt.Errorf("gate: generate: setMembers maps the field %s, which is not tagged gate:%q", name, spec.GateWidening)
		}
	}
	slices.Sort(widening)
	slices.Sort(ungated)
	slices.Sort(recreate)

	fields, err := goFile(`
// wideningFields are the top-level fields of spec.Project tagged
// gate:"widening", by JSON name: a change to one is a change of the
// widening set (Build).
var wideningFields = ` + stringList(widening) + `

// ungatedFields are the top-level fields of spec.Project without the
// gate tag, the committed list of spec.Ungated: RenderDigest strips
// them from the render.
var ungatedFields = ` + stringList(ungated) + `

// recreateFields are the top-level fields of spec.Project tagged
// apply:"recreate": the input of RecreateDigest.
var recreateFields = ` + stringList(recreate) + `
`)
	if err != nil {
		return nil, err
	}

	var rows strings.Builder
	for _, c := range slices.SortedFunc(slices.Values(classes), func(a, b spec.FieldClass) int { return strings.Compare(a.Name, b.Name) }) {
		fmt.Fprintf(&rows, "\t{field: %q, widening: %t, recreate: %t},\n", c.Name, c.Gate == spec.GateWidening, c.Apply == spec.ApplyRecreate)
	}
	flips, err := goFile(`
// flipTable is the I7 flip table (05 5.2): one case per top-level field
// of spec.Project, saying whether a change to it flips the widening
// digest (gate:"widening") and the recreate digest (apply:"recreate").
var flipTable = []flipCase{
` + rows.String() + `}
`)
	if err != nil {
		return nil, err
	}
	return []GeneratedFile{{Path: FieldsPath, Content: fields}, {Path: FlipTablePath, Content: flips}}, nil
}

// goFile returns the Go file of package gate with body, under the
// license lines and the generated header (ADR 0016), formatted.
func goFile(body string) ([]byte, error) {
	src := "// SPDX-FileCopyrightText: 2026 Bruno Venceslau\n// SPDX-License-Identifier: GPL-3.0-only\n\n" +
		cli.GoComment(cli.GeneratedHeader("internal/spec/project.go (the gate and apply tags of Project)")) +
		"\npackage gate\n" + body
	out, err := format.Source([]byte(src))
	if err != nil {
		return nil, fmt.Errorf("gate: generate: %w", err)
	}
	return out, nil
}

// stringList writes names as a Go []string literal, one per line.
func stringList(names []string) string {
	var b strings.Builder
	b.WriteString("[]string{\n")
	for _, n := range names {
		b.WriteString("\t" + strconv.Quote(n) + ",\n")
	}
	b.WriteString("}")
	return b.String()
}
