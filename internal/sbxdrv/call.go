// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"fmt"
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

// slot adds a value of the placeholder p; a value that p refuses is
// the error of the builder, and nothing is added.
func (b *argvBuilder) slot(p Placeholder, value string) *argvBuilder {
	if b.err != nil {
		return b
	}
	if err := p.Check(value); err != nil {
		b.err = fmt.Errorf("sbxdrv: %s: %w", p.Token, err)
		return b
	}
	b.argv = append(b.argv, value)
	b.shape = append(b.shape, p.Token)
	return b
}

// rest adds the words of a command, after "--", as one placeholder in
// the shape: a session matches the argv before "--" and forwards the
// command (10 10.3, Replay only).
func (b *argvBuilder) rest(words []string) *argvBuilder {
	if b.err != nil {
		return b
	}
	if len(words) == 0 {
		b.err = fmt.Errorf("sbxdrv: %s: no command", commandWords.Token)
		return b
	}
	for _, w := range words {
		if err := commandWords.Check(w); err != nil {
			b.err = fmt.Errorf("sbxdrv: %s: %w", commandWords.Token, err)
			return b
		}
	}
	b.argv = append(b.argv, words...)
	b.shape = append(b.shape, commandWords.Token)
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

// Fixture holds the values the examples of the builders are built from:
// the tests build them from fixed ones to compare with the sessions, and
// from the paths and names of a test's own tree to replay the sessions
// through the fake sbx.
type Fixture struct {
	// Dir is the directory that holds sbxenv.yaml.
	Dir string
	// Name is the name of a sandbox.
	Name string
	// Domain is a domain of a network rule.
	Domain string
	// Helper is the host command of a command secret.
	Helper string
	// Julieta is the path of the julieta binary inside the sandbox.
	Julieta string
}

// defaultFixture is the fixture of the shapes: the values the
// placeholders of the sessions stand for.
var defaultFixture = Fixture{
	Dir:     "/abs/root/shop-env",
	Name:    "shop",
	Domain:  "github.com",
	Helper:  "/usr/bin/security find-generic-password -w -s github-shop",
	Julieta: "/home/agent/.local/bin/julieta",
}

// builderEntry is a builder in the registry: its name and one call
// for each shape it can give. The registry is what the scans and the
// session tests read (REQ-0996, REQ-0938).
type builderEntry struct {
	name     string
	examples func(Fixture) []Call
}

// builders is the registry of the argv builders, filled during
// initialization by the file of each builder (rule 4 of docs/plan.md)
// and read-only after.
var builders []builderEntry

// registerBuilder adds a builder to the registry; examples returns a
// call for each shape of its output, built from fixed values.
func registerBuilder(name string, examples func(Fixture) []Call) struct{} {
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

// examples returns every call of every registered builder, built from
// the fixture, sorted by builder name and in the order the builder gave
// them.
func examples(fx Fixture) []Call {
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
		out = append(out, e.examples(fx)...)
	}
	return out
}
