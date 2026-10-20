// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/plan"
)

// TestRegisteredChecks pins the registry of tools/ci: the names of the
// checks in the order of their keys, where each runs, which ones run
// code of the change, and how each runs. A check that leaves fast or
// all, changes its place, or stops judging the commit read before the
// steps fails here.
func TestRegisteredChecks(t *testing.T) {
	type entry struct {
		name            string
		fast, hook, all bool
		runsCode        bool
		how             string
	}
	var got []entry
	for _, c := range check.Registered() {
		how := ""
		switch {
		case c.Steps != nil:
			how = "steps"
		case c.Judge != nil:
			how = "judge"
		case c.After != nil:
			how = "after"
		}
		got = append(got, entry{c.Name, c.Fast, c.Hook, c.All, c.RunsCode, how})
		assert.Equal(t, "tools/ci/internal/"+checkPackage(c.Name), c.Package, "the package of %s", c.Name)
		assert.DirExists(t, filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(c.Package)))
		assert.FileExists(t, "check_"+strings.ReplaceAll(c.Name, " ", "")+".go", "the file that registers %s", c.Name)
		assert.NotEmpty(t, c.Guards, "what %s guards", c.Name)
		assert.NotEmpty(t, c.Usage, "the usage line of %s", c.Name)
	}
	assert.Equal(t, []entry{
		{"format", true, false, true, false, "steps"},
		{"vet", true, false, true, false, "steps"},
		{"fix", true, false, true, false, "steps"},
		{"generated", true, false, true, false, "steps"},
		{"lint", true, false, true, false, "steps"},
		{"vulnerabilities", false, false, true, false, "steps"},
		{"license", false, false, true, false, "steps"},
		{"unit", true, false, true, true, "steps"},
		{"tagged suites", false, false, true, false, "steps"},
		{"race", false, false, true, true, "steps"},
		{"container", false, false, true, true, "steps"},
		{"cross build", false, false, true, false, "steps"},
		{"imports", false, false, true, false, "steps"},
		{"hygiene", true, false, true, false, "judge"},
		{"workflows", true, false, true, false, "judge"},
		{"kits", true, false, true, false, "judge"},
		{"schema", false, false, true, false, "judge"},
		{"reqs", true, false, true, false, "judge"},
		{"plan", true, false, true, false, "judge"},
		{"pushed range", false, true, false, false, "judge"},
		{"coverage", false, false, true, false, "after"},
	}, got)
}

// TestRegisteredChecksAreRowsOfTheStepTable holds each registered check
// to its row of the step table of 10 10.2: the row names the check, and
// the requirement id it registers is one that row holds. A row that
// holds none is registered with none; every row holds one today, so the
// fixture adds a row without one. The fixtures show that a check with an
// id of another row, with an id where its row holds none, and with a row
// the table does not have each fail.
func TestRegisteredChecksAreRowsOfTheStepTable(t *testing.T) {
	rows := stepTableRows(t)
	require.Contains(t, rows, "format, vet", "the step table of 10 10.2 is read")
	assert.Empty(t, stepTableProblems(rows, check.Registered()))
	rows["no id"] = nil
	assert.Equal(t, []string{
		`a: REQ-0824 is not an id of the row "lint"`,
		`b: the row "no id" holds no id, and the check names REQ-0882`,
		`c: "nonesuch" is no row of the step table`,
	}, stepTableProblems(rows, []check.Check{
		{Name: "a", Row: "lint", Req: "REQ-0824"},
		{Name: "b", Row: "no id", Req: "REQ-0882"},
		{Name: "c", Row: "nonesuch", Req: "REQ-0823"},
		{Name: "d", Row: "lint", Req: "REQ-0823"},
	}))
}

// stepTableProblems returns what is wrong with the rows the checks
// name, against rows, the ids of each row of the step table by its step
// cell.
func stepTableProblems(rows map[string][]string, checks []check.Check) []string {
	var problems []string
	for _, c := range checks {
		ids, ok := rows[c.Row]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: %q is no row of the step table", c.Name, c.Row))
		case len(ids) == 0 && c.Req != "":
			problems = append(problems, fmt.Sprintf("%s: the row %q holds no id, and the check names %s", c.Name, c.Row, c.Req))
		case len(ids) > 0 && !slices.Contains(ids, c.Req):
			problems = append(problems, fmt.Sprintf("%s: %s is not an id of the row %q", c.Name, c.Req, c.Row))
		}
	}
	return problems
}

// stepTableRows reads the step table of 10 10.2, the table whose header
// is "| Step | Command | In `fast` |", and returns the requirement ids
// each row holds, by its step cell.
func stepTableRows(t *testing.T) map[string][]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), "docs", "spec", "10-testing-style.md"))
	require.NoError(t, err)
	_, rest, found := strings.Cut(string(b), "\n| Step | Command | In `fast` |\n|---|---|---|\n")
	require.True(t, found, "the header of the step table")
	id := regexp.MustCompile(`REQ-[0-9]{4}`)
	rows := map[string][]string{}
	for line := range strings.SplitSeq(rest, "\n") {
		if !strings.HasPrefix(line, "| ") {
			break
		}
		step, cells, _ := strings.Cut(strings.TrimPrefix(line, "| "), " | ")
		rows[step] = id.FindAllString(cells, -1)
	}
	return rows
}

// TestChecksPageIsCurrent checks that the committed inventory of the
// checks is the one the registry gives; "generated" fails it too, and
// this names the file.
func TestChecksPageIsCurrent(t *testing.T) {
	got, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(check.ChecksPath)))
	require.NoError(t, err)
	assert.Equal(t, string(check.ChecksPage(check.Registered())), string(got), "run go generate ./...")
}

// fixtureCheck and fixturePackage are the two files that add the check
// of TestAFixtureCheckIsAddedByTwoPaths. The check judges the tree, and
// its order key puts it after unit; it also adds a subcommand that
// prints the checks of the registry and the steps of fast, by name.
const (
	fixtureCheck = `package main

import (
	"context"
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/fixture"
)

func init() {
	check.Register(check.Check{
		Name:    "fixture",
		Guards:  "a fixture",
		Row:     "lint",
		Req:     "REQ-0823",
		Usage:   "go run ./tools/ci fixture",
		Fast:    true,
		All:     true,
		Order:   75,
		Package: "tools/ci/internal/fixture",
		Steps:   fixture.Steps,
		Commands: []check.Command{{Name: "fixture", Usage: []string{"fixture"}, Run: func(ctx context.Context, e check.Env, _ []string) (bool, error) {
			for _, c := range check.Registered() {
				fmt.Fprintln(e.Stdout, "check", c.Name)
			}
			for _, s := range fastSteps(ctx, e.Dir, check.Tools{}) {
				fmt.Fprintln(e.Stdout, "step", s.Name)
			}
			return true, nil
		}}},
	})
}
`
	fixturePackage = `// Package fixture is the check of a test.
package fixture

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Steps returns the step of the fixture.
func Steps(context.Context, check.Plan) []check.Step {
	return []check.Step{{Name: "fixture"}}
}
`
)

// TestAFixtureCheckIsAddedByTwoPaths builds tools/ci with two files
// added, tools/ci/check_fixture.go and tools/ci/internal/fixture/, and
// no file edited: the go command reads them through an overlay
// (https://pkg.go.dev/cmd/go#hdr-Build_and_test_caching, -overlay). The
// check they add is in the registry by its order key, between unit and
// race, and in the usage text, and as a check that judges the tree it
// runs before unit in fast, whatever its key says.
func TestAFixtureCheckIsAddedByTwoPaths(t *testing.T) {
	root := citest.ModuleRoot(t)
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	dir := t.TempDir()
	files := map[string]string{
		filepath.Join(root, "tools", "ci", "check_fixture.go"):                  fixtureCheck,
		filepath.Join(root, "tools", "ci", "internal", "fixture", "fixture.go"): fixturePackage,
	}
	replace := map[string]string{}
	for path, src := range files {
		require.NoFileExists(t, path, "the overlay adds the file")
		copied := filepath.Join(dir, filepath.Base(path))
		require.NoError(t, os.WriteFile(copied, []byte(src), 0o600))
		replace[path] = copied
	}
	overlay, err := json.Marshal(map[string]any{"Replace": replace})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "overlay.json"), overlay, 0o600))
	bin := filepath.Join(dir, "ci")
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{goBin, "build", "-overlay=" + filepath.Join(dir, "overlay.json"), "-o", bin, "./tools/ci"},
		Dir:   root,
		Env:   check.StepEnv(filepath.Dir(goBin)),
	})
	require.NoError(t, err, "%s", out)

	out, err = procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{bin, "fixture"},
		Dir:   root,
		Env:   []string{"PATH=" + os.Getenv("PATH")},
	})
	require.NoError(t, err, "%s", out)
	var checks, steps []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		kind, name, _ := strings.Cut(line, " ")
		if kind == "check" {
			checks = append(checks, name)
		} else {
			steps = append(steps, name)
		}
	}
	var registered []string
	for _, c := range check.Registered() {
		registered = append(registered, c.Name)
	}
	unit := slices.Index(registered, "unit")
	require.GreaterOrEqual(t, unit, 0)
	assert.Equal(t, append(append(registered[:unit+1:unit+1], "fixture"), registered[unit+1:]...), checks, "the fixture's key puts it between unit and race")
	assert.Equal(t, []string{"format", "vet", "fix linux/amd64", "fix linux/arm64", "fix darwin/amd64", "fix darwin/arm64", "generated", "lint linux/amd64", "lint boundaries linux/amd64", "lint linux/arm64", "lint boundaries linux/arm64", "lint darwin/amd64", "lint boundaries darwin/amd64", "lint darwin/arm64", "lint boundaries darwin/arm64", "fixture", "unit"}, steps, "a check that judges the tree runs before unit")

	out, err = procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{bin},
		Dir:   root,
		Env:   []string{"PATH=" + os.Getenv("PATH")},
	})
	require.Error(t, err)
	assert.Contains(t, string(out), "  go run ./tools/ci fixture\n", "the usage text names its subcommand")
}

// checkPackage is the directory below tools/ci/internal of the package
// of the check of that name (REQ-0121): its name without spaces, except
// for the generated check, whose package is drift.
func checkPackage(name string) string {
	if name == "generated" {
		return "drift"
	}
	return strings.ReplaceAll(name, " ", "")
}

// TestGeneratedOutputsAreOwnedByNoTask holds plan.Generated, the
// generated outputs of rule 1 of docs/plan.md that the overlap rule of
// the plan check leaves out, to the files every registered generator
// writes: each one matches a glob of the list.
func TestGeneratedOutputsAreOwnedByNoTask(t *testing.T) {
	root, err := os.OpenRoot(citest.ModuleRoot(t))
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	var paths []string
	for _, g := range check.Generators() {
		outs, err := g.Outputs(root)
		require.NoError(t, err, g.Name)
		for _, o := range outs {
			paths = append(paths, o.Path)
		}
	}
	require.NotEmpty(t, paths)
	for _, p := range paths {
		assert.True(t, slices.ContainsFunc(plan.Generated, func(g string) bool { return askfirst.GlobMatch(g, p) }), "%s is in no glob of plan.Generated", p)
	}
}
