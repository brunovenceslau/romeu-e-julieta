// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package scenarios is the shared part of the journeys of 09 (REQ-0724):
// the table of command lines each journey owns, the scenario a journey
// function builds from it, and the world a scenario runs in. A journey
// is a package of its own, e2e/scenarios/<journey>, with a commands.yaml
// (REQ-0730) and a function that reads its lines from that table; this
// package finds every such file by a glob of the directory, so a new
// journey adds a directory and edits no import list. It reads the files
// from the tree, not from an embed: go:embed is allowed below
// internal/catalog only (ADR 0007, rule 8). A test runs in a directory
// of the module, which is where the tree is found from.
//
// A scenario is run by the host suite against real sbx and by the CI
// suite against the fake (REQ-0931): both call the same function, and
// only the World differs. The package needs no sbx fact, so the tasks
// that add a julieta-side journey function wait for no probe.
package scenarios

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/moduleroot"
)

// dir returns the directory of this package in the tree the process
// runs in.
func dir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := moduleroot.Find(wd)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "e2e", "scenarios"), nil
}

// Journeys returns the names of the journeys that have a commands.yaml,
// sorted: the directories of this package.
func Journeys() ([]string, error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(d, "*", "commands.yaml"))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = filepath.Base(filepath.Dir(f))
	}
	slices.Sort(names)
	return names, nil
}

// Table is the commands.yaml of one journey: scenario id to ordered
// command lines (ADR 0001, rule 13). It remembers which lines a scenario
// function read, so that a line no function reads fails the journey's own
// test.
type Table struct {
	journey string
	lines   map[string][]string
	read    map[string][]bool
}

// Load reads the commands.yaml of a journey (a name Journeys returns).
func Load(journey string) (*Table, error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(d, journey, "commands.yaml"))
	if err != nil {
		return nil, err
	}
	return Parse(journey, data)
}

// Parse reads the text of a commands.yaml. Each line is a command line
// of romeu or julieta, as a guide shows it: words split at white space,
// with a <placeholder> where the journey binds a value.
func Parse(journey string, data []byte) (*Table, error) {
	lines := map[string][]string{}
	if err := yaml.Unmarshal(data, &lines); err != nil {
		return nil, fmt.Errorf("scenarios: %s/commands.yaml: %w", journey, err)
	}
	t := &Table{journey: journey, lines: lines, read: map[string][]bool{}}
	for id, ls := range lines {
		t.read[id] = make([]bool, len(ls))
		for i, l := range ls {
			if f := strings.Fields(l); len(f) < 2 || (f[0] != "romeu" && f[0] != "julieta") {
				return nil, fmt.Errorf("scenarios: %s/commands.yaml: %s line %d %q is not a romeu or julieta command", journey, id, i+1, l)
			}
		}
	}
	return t, nil
}

// Line returns line i (from 0) of the scenario id and marks it read. A
// line the table lacks is the empty string, which Scenario.Check refuses
// in a step that runs one.
func (t *Table) Line(id string, i int) string {
	ls := t.lines[id]
	if i < 0 || i >= len(ls) {
		return ""
	}
	t.read[id][i] = true
	return ls[i]
}

// Unread returns the lines no function has read, as "<id> <n>: <line>"
// (n from 1), in the order of the table.
func (t *Table) Unread() []string {
	var out []string
	for _, id := range slices.Sorted(maps.Keys(t.lines)) {
		for i, l := range t.lines[id] {
			if !t.read[id][i] {
				out = append(out, fmt.Sprintf("%s %d: %s", id, i+1, l))
			}
		}
	}
	return out
}

// Commands returns every line of the table, for the test that holds the
// definitions against the journeys.
func (t *Table) Commands() []string {
	var out []string
	for _, id := range slices.Sorted(maps.Keys(t.lines)) {
		out = append(out, t.lines[id]...)
	}
	return out
}

// Vars binds the <placeholder> tokens of a line to values, by name
// without the angle brackets.
type Vars map[string]string

// Bind returns line with each <name> of vars replaced by its value.
func (v Vars) Bind(line string) string {
	for name, value := range v {
		line = strings.ReplaceAll(line, "<"+name+">", value)
	}
	return line
}

// Kind says who acts in a step: the tags of 09 (01 1.7).
type Kind string

// The kinds a step of a scenario has. [O] and [A] steps are
// preconditions or replays a function makes outside a Step.
const (
	Host    Kind = "H" // romeu on the host
	Sbx     Kind = "X" // sbx invoked by romeu
	Julieta Kind = "J" // julieta in the sandbox
)

// Outcome is what a command line did.
type Outcome struct {
	Stdout, Stderr string
	Code           int
}

// World is the machine a scenario runs on. The CI suite implements it
// over a temporary tree and the fake sbx, the host suite over the
// maintainer's machine and real sbx (REQ-0931).
type World interface {
	// Run runs one command line of the journey, binary first, and
	// returns its outcome.
	Run(line string) (Outcome, error)
	// Root is the root of the host tree.
	Root() string
	// Settings returns the bytes of the host settings file.
	Settings() ([]byte, error)
	// State returns the bytes of the file rel in the state directory.
	State(rel string) ([]byte, error)
	// SbxCalls returns the argv of every call romeu made to sbx, in
	// order, without the program name.
	SbxCalls() ([][]string, error)
}

// Step is one numbered step of a journey that romeu, sbx or julieta
// performs.
type Step struct {
	// N is the step's number in 09, such as "3".
	N string
	// Kind is H, X or J.
	Kind Kind
	// Line is the command line the step runs, from the journey's table;
	// empty for a step that runs none.
	Line string
	// Want is the exit status the line ends with: 0 unless a step
	// proves a refusal.
	Want int
	// Assert is the step's assertion: the argv romeu passed to the fake
	// sbx for an [X] step, or the state records the step wrote for an
	// [H] step (REQ-0729). It returns nil when they are as 09 says.
	Assert func(World) error
}

// Scenario is the function of a journey: its steps in order, and the
// steps the host suite covers because the fake cannot perform them.
type Scenario struct {
	// ID is the scenario's id in the table, such as "J1".
	ID string
	// Steps are the steps in the order of 09.
	Steps []Step
	// Omitted are the numbers of the [X] and [H] steps that have no
	// assertion here: attach, a live daemon fetch (REQ-0729).
	Omitted []string
}

// Check returns why the scenario is incomplete: an [H] or [X] step with
// neither an assertion nor an entry in Omitted, an Omitted entry that
// names no step, or a step with a Line the table lacked.
func (s Scenario) Check() error {
	var errs []error
	numbers := map[string]bool{}
	for _, st := range s.Steps {
		numbers[st.N] = true
		if (st.Kind == Host || st.Kind == Sbx) && st.Assert == nil && !slices.Contains(s.Omitted, st.N) {
			errs = append(errs, fmt.Errorf("%s step %s [%s] has no assertion and is not in the omitted steps", s.ID, st.N, st.Kind))
		}
		if st.Assert != nil && slices.Contains(s.Omitted, st.N) {
			errs = append(errs, fmt.Errorf("%s step %s is omitted and has an assertion", s.ID, st.N))
		}
		if st.Kind != Host && st.Kind != Sbx && st.Kind != Julieta {
			errs = append(errs, fmt.Errorf("%s step %s has the kind %q", s.ID, st.N, st.Kind))
		}
	}
	for _, n := range s.Omitted {
		if !numbers[n] {
			errs = append(errs, fmt.Errorf("%s omits step %s, which it does not have", s.ID, n))
		}
	}
	return errors.Join(errs...)
}

// Lines returns the command lines the scenario runs, in order.
func (s Scenario) Lines() []string {
	var out []string
	for _, st := range s.Steps {
		if st.Line != "" {
			out = append(out, st.Line)
		}
	}
	return out
}

// Run runs the steps in order in w: each step's line, which must end
// with the status Want, then its assertion. It stops at the first step
// that fails.
func (s Scenario) Run(w World) error {
	if err := s.Check(); err != nil {
		return err
	}
	for _, st := range s.Steps {
		if st.Line != "" {
			out, err := w.Run(st.Line)
			if err != nil {
				return fmt.Errorf("%s step %s: %w", s.ID, st.N, err)
			}
			if out.Code != st.Want {
				return fmt.Errorf("%s step %s: %q ended with %d, want %d\nstdout: %s\nstderr: %s", s.ID, st.N, st.Line, out.Code, st.Want, out.Stdout, out.Stderr)
			}
		}
		if st.Assert != nil {
			if err := st.Assert(w); err != nil {
				return fmt.Errorf("%s step %s: %w", s.ID, st.N, err)
			}
		}
	}
	return nil
}
