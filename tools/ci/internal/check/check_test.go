// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepsOf is a Steps function that returns one step named for the check.
func stepsOf(name string) func(context.Context, Plan) []Step {
	return func(context.Context, Plan) []Step { return []Step{{Name: name}} }
}

func names(checks []Check) []string {
	var out []string
	for _, c := range checks {
		out = append(out, c.Name)
	}
	return out
}

// TestRegistryOrder checks that the registry returns its checks by
// order key, whatever the order of the calls that added them, and by
// name for an equal key.
func TestRegistryOrder(t *testing.T) {
	var r Registry
	r.Add(Check{Name: "race", Order: 80})
	r.Add(Check{Name: "format", Order: 10})
	r.Add(Check{Name: "vet", Order: 20})
	r.Add(Check{Name: "b", Order: 50})
	r.Add(Check{Name: "a", Order: 50})
	assert.Equal(t, []string{"format", "vet", "a", "b", "race"}, names(r.Checks()))
}

// TestRegistryRefusesADuplicate checks that a second check of one name,
// a second subcommand of one name, a check with no name and a malformed
// requirement id each panic: Register runs from an init, so the binary
// that holds the mistake never starts.
func TestRegistryRefusesADuplicate(t *testing.T) {
	cmd := Command{Name: "hygiene", Usage: []string{"hygiene"}}
	tests := []struct {
		name string
		add  func(r *Registry)
	}{
		{"a check name", func(r *Registry) { r.Add(Check{Name: "vet"}) }},
		{"a subcommand of a check", func(r *Registry) { r.Add(Check{Name: "other", Commands: []Command{cmd}}) }},
		{"a subcommand of no check", func(r *Registry) { r.AddCommand(cmd) }},
		{"a subcommand twice in one check", func(r *Registry) {
			dup := Command{Name: "twice", Usage: []string{"twice"}}
			r.Add(Check{Name: "other", Commands: []Command{dup, dup}})
		}},
		{"no name", func(r *Registry) { r.Add(Check{}) }},
		{"a malformed requirement id", func(r *Registry) { r.Add(Check{Name: "new", Req: "REQ-12"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Registry
			r.Add(Check{Name: "vet", Req: "REQ-0820"})
			r.Add(Check{Name: "hygiene", Commands: []Command{cmd}})
			assert.Panics(t, func() { tt.add(&r) })
			assert.Equal(t, []string{"hygiene", "vet"}, names(r.Checks()), "the registry is unchanged")
		})
	}
}

// TestRunOrderPutsTheJudgesFirst is the case of the acceptance of
// T105: a check that judges the tree and is registered after unit, with
// a larger order key, still runs before unit and race, and the checks
// that run no step are left out.
func TestRunOrderPutsTheJudgesFirst(t *testing.T) {
	var r Registry
	r.Add(Check{Name: "format", Order: 10, Steps: stepsOf("format")})
	r.Add(Check{Name: "generated", Order: 30, Steps: stepsOf("generated")})
	r.Add(Check{Name: "unit", Order: 70, RunsCode: true, Steps: stepsOf("unit")})
	r.Add(Check{Name: "race", Order: 80, RunsCode: true, Steps: stepsOf("race")})
	r.Add(Check{Name: "fixture", Order: 90, Steps: stepsOf("fixture")})
	r.Add(Check{Name: "hygiene", Order: 100})
	assert.Equal(t, []string{"format", "generated", "fixture", "unit", "race"}, names(RunOrder(r.Checks())))
}

// TestUsage checks that the usage text lists the subcommands of no
// check first, in the order they were added, then those of the checks
// by order key.
func TestUsage(t *testing.T) {
	var r Registry
	r.Add(Check{Name: "reqs", Order: 120, Commands: []Command{{Name: "reqs", Usage: []string{"reqs [allocate]"}}}})
	r.Add(Check{Name: "generated", Order: 30, Commands: []Command{
		{Name: "generated", Usage: []string{"generated"}},
		{Name: "generate", Usage: []string{"generate"}},
	}})
	r.AddCommand(Command{Name: "all", Usage: []string{"all"}})
	r.AddCommand(Command{Name: "fast", Usage: []string{"fast [<remote> <url>]"}})
	assert.Equal(t, "usage:\n"+
		"  go run ./tools/ci all\n"+
		"  go run ./tools/ci fast [<remote> <url>]\n"+
		"  go run ./tools/ci generated\n"+
		"  go run ./tools/ci generate\n"+
		"  go run ./tools/ci reqs [allocate]\n", r.Usage())
	cmd, ok := r.Lookup("generate")
	require.True(t, ok)
	assert.Equal(t, "generate", cmd.Name)
	_, ok = r.Lookup("nonesuch")
	assert.False(t, ok)
}

// TestTheRegistryOfTheBinary checks the registry the init of each
// check_<check>.go fills: what Register and RegisterCommand add is what
// Registered, Lookup and Usage give.
func TestTheRegistryOfTheBinary(t *testing.T) {
	run := func(context.Context, Env, []string) (bool, error) { return true, nil }
	RegisterCommand(Command{Name: "all", Usage: []string{"all"}, Run: run})
	Register(Check{Name: "format", Order: 10})
	Register(Check{Name: "reqs", Order: 120, Commands: []Command{{Name: "reqs", Usage: []string{"reqs [allocate]"}, Run: run}}})
	var names []string
	for _, c := range Registered() {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"format", "reqs"}, names)
	cmd, ok := Lookup("reqs")
	require.True(t, ok)
	assert.Equal(t, "reqs", cmd.Name)
	assert.Equal(t, "usage:\n  go run ./tools/ci all\n  go run ./tools/ci reqs [allocate]\n", Usage())
}
