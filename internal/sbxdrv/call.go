// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// Mode says how the standard streams of a call are handled (04 4.1,
// Output).
type Mode int

// The two modes. The zero value names none, and Run refuses it.
const (
	// Captured: stdin is closed and stdout and stderr are captured; the
	// first 4 KiB of stderr enter the details of a failure.
	Captured Mode = iota + 1
	// Interactive: stdin, stdout and stderr are the terminal's, raw,
	// because sbx prompts there (REQ-0270): sbx env run -d [--clone]
	// and the sbx env exec -it of the run layout and the shell.
	Interactive
)

// Call is one sbx call, built by a builder of this package: what to
// pass to sbx, the row of the timeout table it runs under, and how its
// streams are handled. A Call is a value; the runner starts it.
type Call struct {
	// Name is the name of the builder, such as "env-run-create".
	Name string
	// Argv is the arguments of sbx, without the program.
	Argv []string
	// Shape is Argv with a typed placeholder (placeholders.go) in place
	// of each value that varies: the form a session under
	// e2e/testdata/sbx/<version>/ holds.
	Shape []string
	// Class is the row of the timeout table of 04 4.1 the call runs
	// under.
	Class procgroup.Class
	// Mode says whether the call is interactive or captured.
	Mode Mode
}

// argvBuilder builds the argv and the shape of a call side by side.
type argvBuilder struct {
	argv, shape []string
	err         error
}

// lit adds words that are the same in every call of the builder.
func (b *argvBuilder) lit(words ...string) *argvBuilder {
	b.argv = append(b.argv, words...)
	b.shape = append(b.shape, words...)
	return b
}

// call finishes the builder: the Call, or why its values were refused.
// The guards of I4 and I25 run on it (guard.go), so no builder can hand
// out a call they refuse.
func (b *argvBuilder) call(name string, class procgroup.Class, mode Mode) (Call, error) {
	if b.err != nil {
		return Call{}, b.err
	}
	c := Call{Name: name, Argv: b.argv, Shape: b.shape, Class: class, Mode: mode}
	if err := Guard(c); err != nil {
		return Call{}, err
	}
	return c, nil
}

// builderEntry is a builder in the registry: its name and one call
// for each shape it can give. The registry is what the scans and the
// session tests read (REQ-0996, REQ-0938).
type builderEntry struct {
	name     string
	examples func() []Call
}

// builders is the registry of the argv builders, filled during
// initialization by the file of each builder (rule 4 of docs/plan.md)
// and read-only after.
var builders []builderEntry

// registerBuilder adds a builder to the registry; examples returns a
// call for each shape of its output, built from fixed values.
func registerBuilder(name string, examples func() []Call) struct{} {
	builders = append(builders, builderEntry{name: name, examples: examples})
	return struct{}{}
}

// mustCall returns the call of a builder whose values are fixed, for
// the examples of the registry; a builder that refuses its own example
// is a defect found by the first test.
func mustCall(c Call, err error) Call {
	if err != nil {
		panic(err)
	}
	return c
}

// examples returns every call of every registered builder, sorted by
// builder name and in the order the builder gave them.
func examples() []Call {
	entries := slices.Clone(builders)
	slices.SortStableFunc(entries, func(a, b builderEntry) int {
		switch {
		case a.name < b.name:
			return -1
		case a.name > b.name:
			return 1
		}
		return 0
	})
	var out []Call
	for _, e := range entries {
		out = append(out, e.examples()...)
	}
	return out
}
