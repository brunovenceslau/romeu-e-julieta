// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package scenarios_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// TestParseTable reads the lines of a journey and refuses a line that
// names no binary.
func TestParseTable(t *testing.T) {
	tbl, err := scenarios.Parse("jx", []byte("J9:\n  - romeu version\n  - julieta status --json\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"romeu version", "julieta status --json"}, tbl.Commands())

	for name, doc := range map[string]string{
		"a shell line":  "J9:\n  - ls -l\n",
		"a bare binary": "J9:\n  - romeu\n",
		"not a mapping": "- romeu version\n",
	} {
		_, err := scenarios.Parse("jx", []byte(doc))
		assert.Error(t, err, name)
	}
}

// TestTableUnread reports a line no function read, in the table's order.
func TestTableUnread(t *testing.T) {
	tbl, err := scenarios.Parse("jx", []byte("J9:\n  - romeu version\n  - romeu init --config-url <u>\n"))
	require.NoError(t, err)

	assert.Equal(t, "romeu version", tbl.Line("J9", 0))
	assert.Equal(t, "", tbl.Line("J9", 5), "a line the table lacks")
	assert.Equal(t, "", tbl.Line("J8", 0), "a scenario the table lacks")

	assert.Equal(t, []string{"J9 2: romeu init --config-url <u>"}, tbl.Unread())
}

// TestVarsBind replaces each placeholder and leaves an unbound one.
func TestVarsBind(t *testing.T) {
	v := scenarios.Vars{"owner": "me", "repo": "cfg"}
	assert.Equal(t, "romeu init https://h/me/cfg <other>", v.Bind("romeu init https://h/<owner>/<repo> <other>"))
}

// TestScenarioCheck fails an [H] or [X] step with neither an assertion
// nor an entry in the omitted list (REQ-0729), and a malformed list.
func TestScenarioCheck(t *testing.T) {
	ok := func(scenarios.World) error { return nil }
	for _, tc := range []struct {
		name string
		s    scenarios.Scenario
		want string
	}{
		{"asserted", scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{{N: "1", Kind: scenarios.Host, Assert: ok}}}, ""},
		{"omitted", scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{{N: "1", Kind: scenarios.Sbx}}, Omitted: []string{"1"}}, ""},
		{"a julieta step needs none", scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{{N: "1", Kind: scenarios.Julieta}}}, ""},
		{"neither", scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{{N: "2", Kind: scenarios.Sbx}}}, "step 2 [X] has no assertion"},
		{"both", scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{{N: "1", Kind: scenarios.Host, Assert: ok}}, Omitted: []string{"1"}}, "omitted and has an assertion"},
		{"omits no step", scenarios.Scenario{ID: "J9", Omitted: []string{"7"}}, "omits step 7"},
		{"unknown kind", scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{{N: "1", Kind: "O"}}}, `the kind "O"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.Check()
			if tc.want == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

// fakeWorld records the lines it is asked to run and answers each with
// the outcome it holds.
type fakeWorld struct {
	ran  []string
	out  scenarios.Outcome
	fail error
}

func (f *fakeWorld) Run(line string) (scenarios.Outcome, error) {
	f.ran = append(f.ran, line)
	return f.out, f.fail
}
func (f *fakeWorld) Root() string                  { return "/root" }
func (f *fakeWorld) Settings() ([]byte, error)     { return nil, errors.New("none") }
func (f *fakeWorld) State(string) ([]byte, error)  { return nil, errors.New("none") }
func (f *fakeWorld) SbxCalls() ([][]string, error) { return nil, nil }

// TestScenarioRun runs the lines in order, then each assertion, and
// stops at the first failure.
func TestScenarioRun(t *testing.T) {
	asserted := 0
	s := scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{
		{N: "1", Kind: scenarios.Host, Line: "romeu version", Assert: func(scenarios.World) error { asserted++; return nil }},
		{N: "2", Kind: scenarios.Host, Line: "romeu version --json", Assert: func(scenarios.World) error { return errors.New("wrong record") }},
		{N: "3", Kind: scenarios.Host, Line: "romeu never", Assert: func(scenarios.World) error { return nil }},
	}}
	w := &fakeWorld{}

	err := s.Run(w)

	assert.ErrorContains(t, err, "J9 step 2: wrong record")
	assert.Equal(t, []string{"romeu version", "romeu version --json"}, w.ran)
	assert.Equal(t, 1, asserted)
	assert.Equal(t, []string{"romeu version", "romeu version --json", "romeu never"}, s.Lines())
}

// TestScenarioRunExitStatus: a line must end with the status the step
// wants, and a run that cannot start is its error.
func TestScenarioRunExitStatus(t *testing.T) {
	step := scenarios.Step{N: "1", Kind: scenarios.Julieta, Line: "romeu version", Want: 2}
	s := scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{step}}

	assert.ErrorContains(t, s.Run(&fakeWorld{out: scenarios.Outcome{Code: 0, Stderr: "boom"}}), "ended with 0, want 2")
	assert.NoError(t, s.Run(&fakeWorld{out: scenarios.Outcome{Code: 2}}))
	assert.ErrorContains(t, s.Run(&fakeWorld{fail: errors.New("no binary")}), "no binary")
	bad := scenarios.Scenario{ID: "J9", Steps: []scenarios.Step{{N: "1", Kind: scenarios.Sbx}}}
	assert.ErrorContains(t, bad.Run(&fakeWorld{}), "no assertion")
}

// TestJourneysAreTheDirectories finds every journey that has a table.
func TestJourneysAreTheDirectories(t *testing.T) {
	names, err := scenarios.Journeys()
	require.NoError(t, err)
	assert.Contains(t, names, "j1")
	for _, n := range names {
		_, err := scenarios.Load(n)
		assert.NoError(t, err, n)
	}
}

// TestEveryRomeuCommandHasAJourneyLine is the second test of T041 (rule
// 5 of docs/plan.md): a command of the definitions of romeu either has a
// line in some journey's commands.yaml or declares in its own definition
// that no journey runs it, so a missing line is found at the task that
// adds the command. A table line must also name a command that exists,
// and a command that declares no journey must have no line.
func TestEveryRomeuCommandHasAJourneyLine(t *testing.T) {
	names, err := scenarios.Journeys()
	require.NoError(t, err)
	lines := map[string]bool{} // "romeu init" -> a journey runs it
	for _, n := range names {
		tbl, err := scenarios.Load(n)
		require.NoError(t, err)
		for _, l := range tbl.Commands() {
			f := strings.Fields(l)
			bin := f[0]
			var def *cli.Command
			for _, c := range cli.Commands(bin) {
				if f[1] == c.Name && (c.Sub == "" || (len(f) > 2 && f[2] == c.Sub)) {
					def = &c
					break
				}
			}
			if !assert.NotNilf(t, def, "%s: %q names no command of %s", n, l, bin) {
				continue
			}
			lines[bin+" "+def.Line()] = true
		}
	}
	for _, c := range cli.Commands("romeu") {
		key := "romeu " + c.Line()
		switch {
		case lines[key] && c.NoJourney != "":
			t.Errorf("%s declares that no journey runs it (%q) and a commands.yaml has a line for it", key, c.NoJourney)
		case !lines[key] && c.NoJourney == "":
			t.Errorf("%s has no line in any e2e/scenarios/<journey>/commands.yaml and does not declare NoJourney", key)
		}
	}
}
